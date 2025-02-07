package model

import (
	"context"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
)

type App struct { //структура приложеия, используется в пакете app и servicies
	Ctx   context.Context //
	Repo  *Repository     //
	Cache map[string]User //карта, хранящая User сткуртуру
}

type NotifType struct {
	Chat_id int
	Mess_id int
	Text    string
	Sent_at time.Time
	User_id int
	Avatar  string
	Name    string
}

type Repository struct { // используется в пакете app и servicies
	Pool *pgxpool.Pool
}

type User struct { // используется в пакете app и servicies
	Id            int       `json:"id" db:"id"`
	Password_hash string    `json:"password_hash" db:"password_hash"`
	Email         string    `json:"email" db:"email"`
	Phone_number  string    `json:"phone_number" db:"phone_number"`
	Created_at    time.Time `json:"created_at" db:"created_at"`
	Updated_at    time.Time `json:"updated_at" db:"updated_at"`
	Avatar_path   string    `json:"avatar_path" db:"avatar_path"`
	User_type     int       `json:"user_type" db:"user_type"`
	User_role     int       `json:"user_role" db:"user_role"`
}

type Response struct { // служит для json ответов
	Status  string `json:"status"`
	Data    string `json:"data,omitempty"`
	Message string `json:"message"`
}
