package audioplayer

type AudioPlayer interface {
	Start(numChannels int, bytesPerSample int, sampleRate int) error
	Stop()
	NumOutputChannels() int
	Write(data []byte) error
}
