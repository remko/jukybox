package audioplayer

type AudioPlayer interface {
	Start(numChannels int, bytesPerSample int, sampleRate int, isFloatPlanar bool, encoding string) error
	Stop()
	NumOutputChannels() int
	Write(data []byte) error
}

const PCMEncoding = "pcm"
