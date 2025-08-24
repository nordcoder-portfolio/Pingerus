package kafka

import "github.com/segmentio/kafka-go"

type mapCarrierHeaders map[string]string

func (m mapCarrierHeaders) Get(k string) string { return m[k] }
func (m mapCarrierHeaders) Set(k, v string)     { m[k] = v }
func (m mapCarrierHeaders) Keys() []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}
func (m mapCarrierHeaders) ToKafka() []kafka.Header {
	hs := make([]kafka.Header, 0, len(m))
	for k, v := range m {
		hs = append(hs, kafka.Header{Key: k, Value: []byte(v)})
	}
	return hs
}

type mapCarrierFromKafka []kafka.Header

func (h mapCarrierFromKafka) Get(k string) string {
	for _, x := range h {
		if x.Key == k {
			return string(x.Value)
		}
	}
	return ""
}
func (h mapCarrierFromKafka) Set(string, string) {}
func (h mapCarrierFromKafka) Keys() []string {
	ks := make([]string, 0, len(h))
	for _, x := range h {
		ks = append(ks, x.Key)
	}
	return ks
}
