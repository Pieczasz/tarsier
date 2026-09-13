package sarama

type SyncProducer struct{}

type ProducerMessage struct {
	Topic   string
	Value   string
	Headers []RecordHeader
}

type RecordHeader struct {
	Key   []byte
	Value []byte
}

func (p *SyncProducer) SendMessage(msg ProducerMessage) (int32, int64, error) {
	return 0, 0, nil
}

func (p *SyncProducer) SendMessages(msgs []ProducerMessage) error { return nil }
