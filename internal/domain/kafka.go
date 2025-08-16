package domain

type EventPublisher interface {
	PublishCheckRequest(checkID int64) error
	PublishStatusChange(checkID int64, old, new bool) error
}
