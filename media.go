package jukybox

import (
	"github.com/remko/go-mkvparse"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type Chapter struct {
	title string
	start time.Duration
	end   time.Duration
}

const (
	UnknownCodec = iota
	FLAC
)

type Track struct {
	Codec int
}

type MediaFile struct {
	file     string
	title    string
	artist   string
	chapters []Chapter
	duration time.Duration
	tracks   []Track
}

func (m MediaFile) isPassthroughSupported() bool {
	return len(m.tracks) > 0 && m.tracks[0].Codec != FLAC
}

type MediaParser struct {
	duration            float64
	timecodeScale       int64
	currentTagGlobal    bool
	currentTagName      *string
	currentTagValue     *string
	currentChapterStart *int64
	currentChapterEnd   *int64
	currentChapterName  *string
	currentTrackCodec   *string
	currentTrackIsAudio bool
	mediaFile           *MediaFile
}

func (p *MediaParser) HandleMasterBegin(id mkvparse.ElementID, info mkvparse.ElementInfo) (bool, error) {
	if id == mkvparse.TagElement {
		p.currentTagGlobal = true
	} else if id == mkvparse.SimpleTagElement {
		p.currentTagName = nil
		p.currentTagValue = nil
	} else if id == mkvparse.ChapterAtomElement {
		p.currentChapterStart = nil
		p.currentChapterEnd = nil
		p.currentChapterName = nil
	} else if id == mkvparse.TrackEntryElement {
		p.currentTrackCodec = nil
		p.currentTrackIsAudio = false
	}
	return true, nil
}

func (p *MediaParser) HandleMasterEnd(id mkvparse.ElementID, info mkvparse.ElementInfo) error {
	if id == mkvparse.SimpleTagElement && p.currentTagGlobal && p.currentTagName != nil && p.currentTagValue != nil {
		if strings.ToLower(*p.currentTagName) == "artist" {
			p.mediaFile.artist = *p.currentTagValue
		}
	} else if id == mkvparse.ChapterAtomElement {
		if p.currentChapterStart == nil || p.currentChapterEnd == nil {
			log.Printf("%s: Chapter with missing start/end tag\n", p.mediaFile.file)
			return nil
		}
		chapter := Chapter{
			start: time.Duration(*p.currentChapterStart),
			end:   time.Duration(*p.currentChapterEnd),
		}
		if p.currentChapterName != nil {
			chapter.title = *p.currentChapterName
		}
		p.mediaFile.chapters = append(p.mediaFile.chapters, chapter)
	} else if id == mkvparse.TrackEntryElement {
		if p.currentTrackCodec != nil && p.currentTrackIsAudio {
			codec := UnknownCodec
			if *p.currentTrackCodec == "A_FLAC" {
				codec = FLAC
			}
			p.mediaFile.tracks = append(p.mediaFile.tracks, Track{Codec: codec})
		}
	}
	return nil
}

func (p *MediaParser) HandleString(id mkvparse.ElementID, value string, info mkvparse.ElementInfo) error {
	if id == mkvparse.TagNameElement {
		p.currentTagName = &value
	} else if id == mkvparse.TagStringElement {
		p.currentTagValue = &value
	} else if id == mkvparse.TitleElement {
		p.mediaFile.title = value
	} else if id == mkvparse.ChapStringElement {
		p.currentChapterName = &value
	} else if id == mkvparse.CodecIDElement {
		p.currentTrackCodec = &value
	}
	return nil
}

func (p *MediaParser) HandleInteger(id mkvparse.ElementID, value int64, info mkvparse.ElementInfo) error {
	if (id == mkvparse.TagTrackUIDElement || id == mkvparse.TagEditionUIDElement || id == mkvparse.TagChapterUIDElement || id == mkvparse.TagAttachmentUIDElement) && value != 0 {
		p.currentTagGlobal = false
	} else if id == mkvparse.ChapterTimeStartElement {
		p.currentChapterStart = &value
	} else if id == mkvparse.ChapterTimeEndElement {
		p.currentChapterEnd = &value
	} else if id == mkvparse.TrackTypeElement {
		p.currentTrackIsAudio = value == 0x2
	} else if id == mkvparse.TimecodeScaleElement {
		p.timecodeScale = value
	}
	return nil
}

func (p *MediaParser) HandleFloat(id mkvparse.ElementID, value float64, info mkvparse.ElementInfo) error {
	if id == mkvparse.DurationElement {
		p.duration = value
	}
	return nil
}

func (p *MediaParser) HandleDate(id mkvparse.ElementID, value time.Time, info mkvparse.ElementInfo) error {
	return nil
}

func (p *MediaParser) HandleBinary(id mkvparse.ElementID, value []byte, info mkvparse.ElementInfo) error {
	return nil
}

var audioFileRE = regexp.MustCompile(`(?i)\.mk[av]$`)

func parseFile(path string) (*MediaFile, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	handler := MediaParser{
		duration:      -1.0,
		timecodeScale: 1000000,
		mediaFile: &MediaFile{
			file:     path,
			chapters: []Chapter{},
			duration: -1,
		},
	}
	err = mkvparse.ParseSections(file, []mkvparse.ElementID{mkvparse.InfoElement, mkvparse.TagsElement, mkvparse.ChaptersElement, mkvparse.TracksElement}, &handler)
	if err != nil {
		return nil, err
	}

	if handler.duration >= 0 {
		handler.mediaFile.duration = time.Duration(int64(handler.duration * float64(handler.timecodeScale)))
	} else {
		handler.mediaFile.duration = -1
	}
	return handler.mediaFile, nil
}

func GetMedia(sourceDirs []string) []*MediaFile {
	mediaFiles := []*MediaFile{}
	for _, sourceDir := range sourceDirs {
		err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				log.Printf("Error walking %s: %v", path, err)
				return nil
			}
			if !info.Mode().IsRegular() || !audioFileRE.MatchString(path) {
				return nil
			}
			file, err := parseFile(path)
			if err != nil {
				log.Printf("Error loading %s: %v", path, err)
			} else {
				if len(file.tracks) > 0 {
					mediaFiles = append(mediaFiles, file)
				} else {
					log.Printf("%s: File contained no tracks\n", path)
				}
			}
			return nil
		})
		if err != nil {
			log.Print(err)
		}
	}
	return mediaFiles
}
