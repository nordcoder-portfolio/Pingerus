package domain

import (
	"time"
)

type User struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	Password  string    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UserRepo interface {
	Create(u *User) error
	GetByID(id int64) (*User, error)
	GetByEmail(email string) (*User, error)
	Update(u *User) error
}
