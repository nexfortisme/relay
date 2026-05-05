package attachments

type UploadedFile struct {
	Name        string
	ContentType string
	Data        []byte
}

type chunk struct {
	Source string
	Index  int
	Text   string
	Score  int
}

type PromptOptions struct {
	MaxFileBytes  int
	MaxImageBytes int
}