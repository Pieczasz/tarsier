package kgo

type Client struct{}

type Record struct {
	Topic   string
	Value   []byte
	Headers []RecordHeader
}

type RecordHeader struct {
	Key   string
	Value []byte
}

func (c *Client) Produce(ctx interface{}, r *Record, promise interface{}) {}

// ProduceSync is value-shaped so analysistest can pass a composite lit.
func (c *Client) ProduceSync(ctx interface{}, rs ...Record) error { return nil }
