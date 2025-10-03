package speechkit

// RecognitionRequest represents request to start recognition
type RecognitionRequest struct {
	Config RecognitionConfig `json:"config"`
	Audio  AudioSource       `json:"audio"`
}

// RecognitionConfig holds recognition parameters
type RecognitionConfig struct {
	Specification Specification `json:"specification"`
}

// Specification defines audio and recognition parameters
type Specification struct {
	LanguageCode      string `json:"languageCode"`
	Model             string `json:"model"`
	AudioEncoding     string `json:"audioEncoding"`
	SampleRateHertz   int    `json:"sampleRateHertz"`
	AudioChannelCount int    `json:"audioChannelCount"`
	ProfanityFilter   bool   `json:"profanityFilter"`
	LiteratureText    bool   `json:"literatureText"`
}

// AudioSource specifies location of audio file
type AudioSource struct {
	URI string `json:"uri"`
}

// OperationResponse represents Yandex Cloud operation response
type OperationResponse struct {
	ID          string                 `json:"id"`
	Description string                 `json:"description"`
	CreatedAt   string                 `json:"createdAt"`
	CreatedBy   string                 `json:"createdBy"`
	ModifiedAt  string                 `json:"modifiedAt"`
	Done        bool                   `json:"done"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Response    interface{}            `json:"response,omitempty"`
	Error       *OperationError        `json:"error,omitempty"`
}

// OperationError represents error in operation
type OperationError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// RecognitionResult represents final recognition result
type RecognitionResult struct {
	Chunks []Chunk `json:"chunks"`
}

// Chunk represents one chunk of recognized text
type Chunk struct {
	Alternatives []Alternative `json:"alternatives"`
	ChannelTag   string        `json:"channelTag,omitempty"`
	StartTimeMs  int64         `json:"startTimeMs,omitempty"`
	EndTimeMs    int64         `json:"endTimeMs,omitempty"`
}

// Alternative represents one recognition variant
type Alternative struct {
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence,omitempty"`
	Words      []Word  `json:"words,omitempty"`
}

// Word represents single word with timing
type Word struct {
	StartTimeMs int64   `json:"startTimeMs"`
	EndTimeMs   int64   `json:"endTimeMs"`
	Word        string  `json:"word"`
	Confidence  float64 `json:"confidence"`
}
