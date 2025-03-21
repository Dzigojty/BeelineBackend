package database

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"mime/multipart"
	function "myproject/internal"
	"myproject/internal/jwt"
	"myproject/internal/model"
	"net/http"
	"net/smtp"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/rs/zerolog"
)

type Repository struct { // используется в пакете app и servicies
	Pool *pgxpool.Pool
}

type MyRepository struct {
	App *Repository
}

type SignupHandler struct {
	RedisClient *redis.Client
	Logger      zerolog.Logger
	CodeNum     int
}

func NewRepo(Ctx context.Context, dbpool *pgxpool.Pool) *MyRepository {
	return &MyRepository{&Repository{}}
}

type LegalUser struct {
	Id            int       `json:"id" db:"id"`
	Password_hash string    `json:"password_hash" db:"password_hash"`
	Email         string    `json:"email" db:"email"`
	Phone_number  string    `json:"phone_number" db:"phone_number"`
	Created_at    time.Time `json:"created_at" db:"created_at"`
	Updated_at    time.Time `json:"updated_at" db:"updated_at"`
	Avatar_path   string    `json:"avatar_path" db:"avatar_path"`
	User_type     int       `json:"user_type" db:"user_type"`
	User_role     int       `json:"user_role" db:"user_role"`

	Ind_num_taxp    int    `json:"ind_num_taxp" db:"ind_num_taxp"`
	Name_of_company string `json:"name_of_company" db:"name_of_company"`
	Address_name    string `json:"address_name" db:"address_name"`
}

type NaturUser struct {
	Id            int       `json:"id" db:"id"`
	Password_hash string    `json:"password_hash" db:"password_hash"`
	Email         string    `json:"email" db:"email"`
	Phone_number  string    `json:"phone_number" db:"phone_number"`
	Created_at    time.Time `json:"created_at" db:"created_at"`
	Updated_at    time.Time `json:"updated_at" db:"updated_at"`
	Avatar_path   string    `json:"avatar_path" db:"avatar_path"`
	User_type     int       `json:"user_type" db:"user_type"`
	User_role     int       `json:"user_role" db:"user_role"`

	Name       string `json:"name" db:"name"`
	Surname    string `json:"surname" db:"surname"`
	Patronymic string `json:"patronymic" db:"patronymic"`
}

type FileUploadRequest struct {
	Filename string `json:"filename"`
	Filetype string `json:"filetype"`
	Data     []byte `json:"data"`
}

func UploadAvatar(rw http.ResponseWriter, imageBase64, directory, user_id, index string) (error, bool, string) {
	// Проверяем, содержит ли строка базовые метаданные
	if strings.HasPrefix(imageBase64, "data:image/png;base64,") {
		// Извлекаем данные после запятой
		commaIndex := strings.Index(imageBase64, ",")
		if commaIndex == -1 {
			http.Error(rw, "Invalid base64 data", http.StatusBadRequest)
			return fmt.Errorf("invalid base64 data"), false, ""
		}
		imageBase64 = imageBase64[commaIndex+1:]
	}

	// Декодируем данные base64
	data, err := base64.StdEncoding.DecodeString(imageBase64)
	if err != nil {
		http.Error(rw, fmt.Sprintf("Error decoding base64: %v", err), http.StatusInternalServerError)
		return err, false, ""
	}

	fmt.Println("Decoded data length:", len(data))

	// Генерируем уникальное имя файла
	fileName := function.GenerateFileName("png", user_id, index)

	// Полный путь до файла
	filePath := filepath.Join(directory, fileName)

	// Проверяем и создаём директорию, если она не существует
	if _, err := os.Stat(directory); os.IsNotExist(err) {
		err = os.MkdirAll(directory, 0755) // Создаем директорию с правами 0755
		if err != nil {
			http.Error(rw, fmt.Sprintf("Error creating directory %s: %v", directory, err), http.StatusInternalServerError)
			return err, false, ""
		}
	}

	// Создаем файл на сервере для сохранения декодированного файла
	fmt.Println("Creating file at path:", filePath)
	dst, err := os.Create(filePath) // Используем безопасное имя файла
	if err != nil {
		http.Error(rw, fmt.Sprintf("Error creating file %s: %v", filePath, err), http.StatusInternalServerError)
		return err, false, ""
	}
	defer dst.Close()

	// Записываем данные в файл
	if _, err := dst.Write(data); err != nil {
		http.Error(rw, fmt.Sprintf("Error writing to file %s: %v", fileName, err), http.StatusInternalServerError)
		return err, false, ""
	}

	return nil, true, filePath
}

// Функция для загрузки файла и возврата его в формате base64
func DownloadFile(filePath string) (string, error) {
	// Открываем файл
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("ошибка при открытии файла %s: %v\n", filePath, err)
		return "", err
	}

	// Читаем данные файла
	fileData, err := io.ReadAll(file)
	if err != nil {
		fmt.Printf("ошибка при чтении файла %s: %v\n", filePath, err)
		return "", err
	}

	// Кодируем данные в base64
	encoded := base64.StdEncoding.EncodeToString(fileData)
	defer file.Close()

	return encoded, nil
}

func DeleteImage(rw http.ResponseWriter, file_path string) (bool, error) {
	// Удаляем файл
	err := os.Remove(file_path)
	if err != nil {
		http.Error(rw, fmt.Sprintf("Unable to delete file %s: %v", file_path, err), http.StatusInternalServerError)
		return false, err
	}

	return true, nil
}

func DeleteImageMass(rw http.ResponseWriter, pwd string, images []string) (bool, error) {
	for _, imageName := range images {
		// Формируем полный путь к файлу
		filePath := pwd + imageName

		// Удаляем файл
		err := os.Remove(filePath)
		if err != nil {
			return false, fmt.Errorf("Unable to delete file %s: %v", imageName, err)
		}
	}
	return true, nil
}

// Функция для удаления одного видеофайла
func DeleteVideo(rw http.ResponseWriter, videoName string, pwd string) (error, bool) {
	// Формируем полный путь к файлу
	filePath := pwd + videoName

	// Удаляем файл с сервера
	err := os.Remove(filePath)
	if err != nil {
		http.Error(rw, fmt.Sprintf("Error deleting file %s: %v", videoName, err), http.StatusInternalServerError)
		return err, false
	}

	return nil, true
}

// Функция для массового удаления множества видеофайлов
func DeleteVideosMass(rw http.ResponseWriter, videoNames []string, pwd string) (error, bool) {
	for _, videoName := range videoNames {
		err, success := DeleteVideo(rw, videoName, pwd)
		if err != nil || !success {
			return err, false
		}
	}
	return nil, true
}

// Функция для обработки запроса на получение конкретного файла (например, file.png) в Base64
func ServeSpecificMediaBase64(rw http.ResponseWriter, req *http.Request, file_path string) string {
	// Проверяем, существует ли файл
	if _, err := os.Stat(file_path); os.IsNotExist(err) {
		// Возвращаем ошибку, если файл не найден
		return "File not found"
	}

	// Читаем содержимое файла
	fileData, err := os.ReadFile(file_path)
	if err != nil {
		// Обработка ошибки чтения файла
		return "Error reading file"
	}

	// Кодируем содержимое файла в Base64
	encodedData := base64.StdEncoding.EncodeToString(fileData)

	// Возвращаем закодированные данные
	return encodedData
}

func errorr(err error) {
	if err != nil {
		err = fmt.Errorf("failed to exec data: %w", err)
		return
	}
}

func (repo *MyRepository) SigLegalUserEmailSQL(ctx context.Context, rep *pgxpool.Pool, rw http.ResponseWriter, r *http.Request, ind_num_taxp int, name_of_company, address_name, email, hashedPassword, Data, file_path string) (err error) {
	type Response struct {
		Status  string `json:"status"`
		Data    int    `json:"data"`
		Message string `json:"message"`
	}
	result, errors := rep.Query(ctx, `
			WITH i AS (
				INSERT INTO Users.users (
					user_type, password_hash, email, avatar_path
				)
				VALUES ($1, $2, $3, $4)
				RETURNING id
			)
			INSERT INTO Users.company_user (
				user_id, ind_num_taxp, name_of_company, address_name
			)
			SELECT i.id, $5, $6, $7 FROM i
			RETURNING user_id;
		`,

		1,
		hashedPassword,
		email,
		file_path,

		ind_num_taxp,
		name_of_company,
		address_name,
	)

	var User_id int
	for result.Next() {
		err := result.Scan(
			&User_id,
		)

		if err != nil {
			fmt.Println(err)
			continue
		}
	}

	if User_id == 0 || errors != nil {
		_, err = DeleteImage(rw, file_path)
		errorr(err)

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(Response{
			Status:  "fatal",
			Message: "Введите корректный и уникальный логин и пароль ",
		})

		return
	} else {
		response := Response{
			Status:  "success",
			Data:    User_id,
			Message: "The user has been successfully registered",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)

		return
	}
}

func (repo *MyRepository) SigLegalUserPhoneSQL(ctx context.Context, rep *pgxpool.Pool, rw http.ResponseWriter, r *http.Request, ind_num_taxp int, name_of_company, address_name, phone_number, hashedPassword, Data, file_path string) (err error) {
	type Response struct {
		Status  string `json:"status"`
		Data    int    `json:"data"`
		Message string `json:"message"`
	}
	result, errors := rep.Query(ctx, `
			WITH i AS (
				INSERT INTO Users.users (
					user_type, password_hash, phone_number, updated_at, avatar_path
				)
				VALUES ($1, $2, $3, $4, $5) 
				RETURNING id
			)
			INSERT INTO Users.company_user (
				user_id, ind_num_taxp, name_of_company, address_name
			)
			SELECT i.id, $6, $7, $8 FROM i
			RETURNING user_id;
		`,
		1,
		hashedPassword,
		phone_number,
		time.Now(),
		file_path,

		ind_num_taxp,
		name_of_company,
		address_name,
	)

	var User_id int
	for result.Next() {
		err := result.Scan(
			&User_id,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}
	}

	if User_id == 0 || errors != nil {
		// _, err = DeleteImage(rw, file_path)
		// errorr(err)

		json.NewEncoder(rw).Encode(Response{
			Status:  "fatal",
			Message: "Введите корректный & уникальный логин и пароль ",
		})

		return
		// } else if errors != nil {
		// 	_, err = DeleteImage(rw, file_path)
		// 	errorr(err)

		// 	rw.WriteHeader(http.StatusOK)
		// 	json.NewEncoder(rw).Encode(Response{
		// 		Status:  "fatal",
		// 		Message: errors.Error(),
		// 	})

		// 	return
		// } else {
	}

	response := Response{
		Status:  "success",
		Data:    User_id,
		Message: "The user has been successfully registered",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)

	return
	// }
}

func (repo *MyRepository) SigNaturUserEmailSQL(
	ctx context.Context,
	rep *pgxpool.Pool,
	rw http.ResponseWriter,
	r *http.Request,
	name,
	surname,
	patronymic,
	email,
	hashedPassword,

	Data,
	file_path string) (err error) {
	result, errors := rep.Query(ctx,
		`
			WITH i AS (
			INSERT INTO Users.users (
				user_type, password_hash, email, updated_at, avatar_path
			) 
			VALUES ($1, $2, $3, $4, $5) 
			RETURNING id
			)
			INSERT INTO Users.individual_user (
				user_id, name, surname, patronymic
			)
			SELECT i.id, $6, $7, $8 FROM i
			RETURNING user_id;
		`,
		1,
		hashedPassword,
		email,
		time.Now(),
		file_path,

		name,
		surname,
		patronymic)
	var User_id int
	for result.Next() {
		err := result.Scan(
			&User_id,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}
	}

	type Response struct {
		Status  string `json:"status"`
		Data    int    `json:"data,omitempty"`
		Message string `json:"message"`
	}
	rw.WriteHeader(http.StatusOK)

	if User_id == 0 || errors != nil {
		_, err = DeleteImage(rw, file_path)
		errorr(err)

		json.NewEncoder(rw).Encode(Response{
			Status:  "fatal",
			Message: "Введите корректный & уникальный логин и пароль ",
		})

		return
	}
	response := Response{
		Status:  "success",
		Data:    User_id,
		Message: "The user has been successfully registered",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)

	return
}

func (repo *MyRepository) SigNaturUserPhoneSQL(
	ctx context.Context,
	rep *pgxpool.Pool,
	rw http.ResponseWriter,
	r *http.Request,
	name,
	surname,
	patronymic,
	phone_number,
	hashedPassword,

	Data,
	file_path string) (err error) {
	result, errors := rep.Query(ctx,
		`
			WITH i AS (
				INSERT INTO Users.users (
					user_type, password_hash, phone_number, updated_at, avatar_path
				) 
				VALUES ($1, $2, $3, $4, $5) 
				RETURNING id
			)
				INSERT INTO Users.individual_user (
					user_id, name, surname, patronymic
				)
			SELECT i.id, $6, $7, $8 FROM i
			RETURNING user_id;
		`,
		1,
		hashedPassword,
		phone_number,
		time.Now(),
		file_path,

		name,
		surname,
		patronymic)
	var User_id int
	for result.Next() {
		err := result.Scan(
			&User_id,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}
	}

	type Response struct {
		Status  string `json:"status"`
		Data    int    `json:"data,omitempty"`
		Message string `json:"message"`
	}
	rw.WriteHeader(http.StatusOK)

	if User_id == 0 || errors != nil {
		_, err = DeleteImage(rw, file_path)
		errorr(err)

		json.NewEncoder(rw).Encode(Response{
			Status:  "fatal",
			Message: "Введите корректный & уникальный логин и пароль ",
		})

		return
	}
	response := Response{
		Status:  "success",
		Data:    User_id,
		Message: "The user has been successfully registered",
	}

	json.NewEncoder(rw).Encode(response)

	return
}

type Login struct {
	Id                        int     `json:"Id"`
	Login                     string  `json:"Login"`
	Name                      string  `json:"Name"`
	Surname_or_Ind_num        string  `json:"Surname_or_Ind_num"`
	Patronomic_or_Addres_name string  `json:"Patronomic_or_Addres_name"`
	Rating                    float32 `json:"Rating"`
	Total_balance             int     `json:"Total_balance"`
	User_role                 int     `json:"User_role"`
}

func (repo *MyRepository) CallbackSQL(ctx context.Context, rep *pgxpool.Pool, rw http.ResponseWriter, logger zerolog.Logger, userInfo model.YandexUserInfo) (err error) {
	var userId int

	var u Login

	row := rep.QueryRow(ctx,
		`
		SELECT users.id,
			users.email,
			COALESCE(individual_user.Name::TEXT, company_user.Name_of_company::TEXT) AS Name,
			COALESCE(individual_user.Surname::TEXT, company_user.Ind_num_taxp::TEXT) AS Surname_or_Ind_num,
			COALESCE(individual_user.Patronymic::TEXT, company_user.Address_name::TEXT) AS Patronomic_or_Addres_name,
			users.rating,
			wallets.total_balance,
			users.user_role
		FROM Users.users
			LEFT JOIN Users.individual_user ON users.id = individual_user.user_id
			LEFT JOIN Users.company_user ON users.id = company_user.user_id
			JOIN finance.wallets ON users.id = wallets.user_id
			WHERE email = $1 OR phone_number = $1;
		`,
		userInfo.Login)
	err = row.Scan(
		&u.Id,
		&u.Login,
		&u.Name,
		&u.Surname_or_Ind_num,
		&u.Patronomic_or_Addres_name,
		&u.Rating,
		&u.Total_balance,
		&u.User_role,
	)
	err = row.Scan(
		&userId,
	)

	if userId == 0 {
		response := model.Response{
			Status:  "fatal",
			Message: "Юзер должен указать доп параметры",
		}

		// Лог с контекстом
		logger.Info().
			Str("service", "login").
			Int("port", 8080).
			Msg("Пользователь должен указать доп данные")

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)
		return
	}

	type User struct {
		Information   Login  `json:"Information"`
		JWT           string `json:"JWT"`
		Refresh_token string `json:"Refresh_token"`
	}

	type Response struct {
		Status  string `json:"status"`
		Data    User   `json:"data,omitempty"`
		Message string `json:"message"`
	}

	if err != nil || u.Id == 0 {
		response := Response{
			Status:  "fatal",
			Message: err.Error(),
		}

		// Лог с контекстом
		logger.Info().
			Str("service", "login").
			Int("port", 8080).
			Msg("The user entered an invalid username or password")

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)
		return
	}

	// Генерация JWT токена
	validToken_jwt, err := jwt.GenerateJWT("jwt", u.Id, u.User_role)
	errorr(err)

	// Генерация refresh токена
	refresh_token, err := jwt.GenerateJWT("refresh", u.Id, u.User_role)
	errorr(err)

	user := User{
		Information:   u,
		JWT:           validToken_jwt,
		Refresh_token: refresh_token,
	}

	// Установка куки
	livingTime := 60 * time.Minute
	expiration := time.Now().Add(livingTime)
	cookie := http.Cookie{
		Name:     "token",
		Value:    url.QueryEscape(validToken_jwt),
		Expires:  expiration,
		Path:     "/",             // Убедитесь, что путь корректен
		Domain:   "185.112.83.36", // IP-адрес вашего сервера
		HttpOnly: true,
		Secure:   false, // Для HTTP можно оставить false
		SameSite: http.SameSiteLaxMode,
	}

	// Установка куки
	livingTime = 30 * 24 * time.Hour //не смог найти чего-то получше
	expiration = time.Now().Add(livingTime)
	cookie = http.Cookie{
		Name:     "token",
		Value:    url.QueryEscape(refresh_token),
		Expires:  expiration,
		Path:     "/",             // Убедитесь, что путь корректен
		Domain:   "185.112.83.36", // IP-адрес вашего сервера
		HttpOnly: true,
		Secure:   false, // Для HTTP можно оставить false
		SameSite: http.SameSiteLaxMode,
	}

	_ = cookie

	response := Response{
		Status:  "success",
		Data:    user,
		Message: "You have successfully logged in",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)
	return
}

func (repo *MyRepository) RecoveryPassSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, redisClient *redis.Client, user_id int, passwd string) (err error) {
	request, err := rep.Query(
		ctx,
		`UPDATE users.users SET password_hash = $1 WHERE id = $2 RETURNING id;`,
		passwd,
		user_id)

	errorr(err)

	var id int

	for request.Next() {
		err := request.Scan(
			&id,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}
	}

	type Response struct {
		Status  string `json:"status"`
		Data    int    `json:"data,omitempty"`
		Message string `json:"message"`
	}

	if err != nil || id != 0 {
		response := Response{
			Status:  "success",
			Data:    id,
			Message: "Показано",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)

		return err
	}

	response := Response{
		Status:  "fatal",
		Message: "Не показано",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)

	return err
}

func (repo *MyRepository) LoginSQL(ctx context.Context, rep *pgxpool.Pool, rw http.ResponseWriter, login, hashedPassword string, logger zerolog.Logger) (err error) {
	var u Login

	row := rep.QueryRow(ctx,
		`
		SELECT users.id,
			COALESCE(users.email, 'Нет email'),
			COALESCE(individual_user.Name::TEXT, company_user.Name_of_company::TEXT) AS Name,
			COALESCE(individual_user.Surname::TEXT, company_user.Ind_num_taxp::TEXT) AS Surname_or_Ind_num,
			COALESCE(individual_user.Patronymic::TEXT, company_user.Address_name::TEXT) AS Patronomic_or_Addres_name,
			users.rating,
			wallets.total_balance,
			users.user_role
		FROM Users.users
			LEFT JOIN Users.individual_user ON users.id = individual_user.user_id
			LEFT JOIN Users.company_user ON users.id = company_user.user_id
			JOIN finance.wallets ON users.id = wallets.user_id
			WHERE (email = $1 AND password_hash = $2) 
			OR (phone_number = $1 AND password_hash = $2);`,
		login, hashedPassword)
	err = row.Scan(
		&u.Id,
		&u.Login,
		&u.Name,
		&u.Surname_or_Ind_num,
		&u.Patronomic_or_Addres_name,
		&u.Rating,
		&u.Total_balance,
		&u.User_role,
	)

	errorr(err)

	type User struct {
		Information   Login  `json:"Information"`
		JWT           string `json:"JWT"`
		Refresh_token string `json:"Refresh_token"`
	}

	type Response struct {
		Status  string `json:"status"`
		Data    User   `json:"data,omitempty"`
		Message string `json:"message"`
	}

	if err != nil || u.Id == 0 {
		response := Response{
			Status:  "fatal",
			Message: err.Error(),
		}

		// Лог с контекстом
		logger.Info().
			Str("service", "login").
			Int("port", 8080).
			Msg("The user entered an invalid username or password")

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)
		return
	}

	// Генерация JWT токена
	validToken_jwt, err := jwt.GenerateJWT("jwt", u.Id, u.User_role)
	errorr(err)

	// Генерация refresh токена
	refresh_token, err := jwt.GenerateJWT("refresh", u.Id, u.User_role)
	errorr(err)

	user := User{
		Information:   u,
		JWT:           validToken_jwt,
		Refresh_token: refresh_token,
	}

	// Установка куки
	livingTime := 60 * time.Minute
	expiration := time.Now().Add(livingTime)
	cookie := http.Cookie{
		Name:     "token",
		Value:    url.QueryEscape(validToken_jwt),
		Expires:  expiration,
		Path:     "/",             // Убедитесь, что путь корректен
		Domain:   "185.112.83.36", // IP-адрес вашего сервера
		HttpOnly: true,
		Secure:   false, // Для HTTP можно оставить false
		SameSite: http.SameSiteLaxMode,
	}

	// Установка куки
	livingTime = 30 * 24 * time.Hour //не смог найти чего-то получше
	expiration = time.Now().Add(livingTime)
	cookie = http.Cookie{
		Name:     "token",
		Value:    url.QueryEscape(refresh_token),
		Expires:  expiration,
		Path:     "/",             // Убедитесь, что путь корректен
		Domain:   "185.112.83.36", // IP-адрес вашего сервера
		HttpOnly: true,
		Secure:   false, // Для HTTP можно оставить false
		SameSite: http.SameSiteLaxMode,
	}

	_ = cookie

	response := Response{
		Status:  "success",
		Data:    user,
		Message: "You have successfully logged in",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)
	return
}

func (repo *MyRepository) DisputeChatPanelSQL(ctx context.Context, rep *pgxpool.Pool, rw http.ResponseWriter) (err error) {
	type Chat struct {
		Chat_id   int
		User_1_id int
		Name1     string
		User_2_id int
		Name2     string
		Text      string
		Ad_id     int
	}
	products := []Chat{}

	if err != nil {
		err = fmt.Errorf("failed to exec data: %w", err)

		return
	}

	request, err := rep.Query(
		ctx,
		`
		SELECT DISTINCT ON (chats.id)
			chats.id AS chat_id,
			chats.user_1_id,
			COALESCE(individual_user_1.name, company_user_1.name_of_company) AS user_1_name,
			chats.user_2_id,
			COALESCE(individual_user_2.name, company_user_2.name_of_company) AS user_2_name,
			COALESCE(messages.text, 'Нет сообщений') AS last_message_text,
			chats.ad_id
		FROM chat.chats
		LEFT JOIN users.individual_user AS individual_user_1 
			ON individual_user_1.user_id = chats.user_1_id
		LEFT JOIN users.company_user AS company_user_1 
			ON company_user_1.user_id = chats.user_1_id
		LEFT JOIN users.individual_user AS individual_user_2 
			ON individual_user_2.user_id = chats.user_2_id
		LEFT JOIN users.company_user AS company_user_2 
			ON company_user_2.user_id = chats.user_2_id
		LEFT JOIN chat.messages
			ON messages.chat_id = chats.id
		WHERE chats.have_disput = true
		AND (chats.mediator_id IS NULL)
		AND chats.state = false
		-- Важно!
		ORDER BY chats.id, COALESCE(messages.text, 'Нет сообщений') DESC;  -- или по другому полю, указывающему "последнее" сообщение
		`)

	errorr(err)

	for request.Next() {
		p := Chat{}
		err := request.Scan(
			&p.Chat_id,
			&p.User_1_id,
			&p.Name1,
			&p.User_2_id,
			&p.Name2,
			&p.Text,
			&p.Ad_id,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}

		products = append(products, p)
	}

	type Response struct {
		Status  string `json:"status"`
		Data    []Chat `json:"data,omitempty"`
		Message string `json:"message"`
	}

	if len(products) == 0 {
		val := []Chat{}
		val = append(val, Chat{
			Chat_id:   0,
			User_1_id: 0,
			Name1:     "",
			User_2_id: 0,
			Name2:     "",
			Text:      "",
			Ad_id:     0,
		})

		response := Response{
			Status:  "success",
			Data:    val,
			Message: "Показано",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)

		return
	}

	if err == nil && request != nil {
		response := Response{
			Status:  "success",
			Data:    products,
			Message: "Показано",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)

		return
	}

	response := Response{
		Status:  "fatal",
		Message: "Не показано",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)

	return err
}

func (repo *MyRepository) RecoveryPassWithEmailSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, redisClient *redis.Client, user_id int, passwd, jwt_for_proof string) (err error) {
	type Email struct {
		Email_name string
	}
	products := []Email{}

	if err != nil {
		err = fmt.Errorf("failed to exec data: %w", err)

		return
	}

	request, err := rep.Query(
		ctx,
		`SELECT email FROM users.users 
			WHERE id = $1;`,
		user_id)

	errorr(err)

	for request.Next() {
		p := Email{}
		err := request.Scan(
			&p.Email_name,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}

		products = append(products, p)
	}

	type Data_requst struct {
		Email         string `json:"Email"`
		Jwt_for_proof string `json:"Jwt_for_proof"`
	}

	type Response struct {
		Status  string      `json:"status"`
		Data    Data_requst `json:"data,omitempty"`
		Message string      `json:"message"`
	}

	if err == nil && request != nil && len(products) != 0 {
		// Настройки SMTP-сервера
		smtpHost := "smtp.mail.ru"
		smtpPort := "587"

		// Данные отправителя (ваша почта и пароль приложения)
		senderEmail := "parpatt_test@mail.ru"
		password := "X0h72ndPXchhjWZ4vbyT" // Пароль приложения

		// Получатель
		recipientEmail := products[0].Email_name

		// Сообщение
		subject := "Subject: Восстанови свой passwd !.\n"
		body := "Введи этот код.\n"
		codeNum := 777 // Здесь лучше использовать случайный код
		message := []byte(subject + "\n" + body + strconv.Itoa(codeNum))

		// Авторизация для отправки email
		auth := smtp.PlainAuth("", senderEmail, password, smtpHost)

		// Устанавливаем обычное нешифрованное соединение
		client, err := smtp.Dial(smtpHost + ":" + smtpPort)
		if err != nil {
			log.Fatal(err)
		}

		// Используем команду STARTTLS для начала TLS-сессии
		tlsConfig := &tls.Config{
			InsecureSkipVerify: true, // Это нужно убрать в продакшене
			ServerName:         smtpHost,
		}

		if err = client.StartTLS(tlsConfig); err != nil {
			log.Fatal(err)
		}

		// Старт авторизации
		if err = client.Auth(auth); err != nil {
			log.Fatal(err)
		}

		// Установка адреса отправителя
		if err = client.Mail(senderEmail); err != nil {
			log.Fatal(err)
		}

		// Установка адреса получателя
		if err = client.Rcpt(recipientEmail); err != nil {
			log.Fatal(err)
		}

		// Отправка сообщения
		w, err := client.Data()
		if err != nil {
			log.Fatal(err)
		}

		_, err = w.Write(message)
		if err != nil {
			log.Fatal(err)
		}

		err = w.Close()
		if err != nil {
			log.Fatal(err)
		}

		// Завершение сеанса
		client.Quit()

		type Kesh struct {
			Passwd []byte
			Code   int
		}

		hash := sha256.Sum256([]byte(passwd))

		// Преобразуем структуру kesh в JSON
		keshData, err := json.Marshal(Kesh{Passwd: hash[:], Code: codeNum})
		if err != nil {
			log.Fatal("Ошибка при сериализации структуры kesh:", err)
			http.Error(rw, "Ошибка сервера", http.StatusInternalServerError)
			return err
		}

		// Сохраняем код подтверждения в Redis с TTL 10 минут
		err = redisClient.Set(ctx, jwt_for_proof, keshData, 10*time.Minute).Err()
		if err != nil {
			log.Fatal("Ошибка при сохранении кода в Redis:", err)
			http.Error(rw, "Ошибка сервера", http.StatusInternalServerError)
			return err
		}

		response := Response{
			Status:  "success",
			Data:    Data_requst{Email: products[0].Email_name, Jwt_for_proof: jwt_for_proof},
			Message: "Показано",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)

		return err
	}

	response := Response{
		Status:  "fatal",
		Message: "Не показано",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)

	return err
}

func (repo *MyRepository) RecoveryPassWithPhoneNumSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, redisClient *redis.Client, user_id int, passwd, jwt_for_proof string) (err error) {
	type Phone struct {
		Phone_num string
	}
	products := []Phone{}

	if err != nil {
		err = fmt.Errorf("failed to exec data: %w", err)

		return
	}

	request, err := rep.Query(
		ctx,
		`SELECT phone_number FROM users.users 
			WHERE id = $1;`,
		user_id)

	errorr(err)

	for request.Next() {
		p := Phone{}
		err := request.Scan(
			&p.Phone_num,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}

		products = append(products, p)
	}

	type Data_requst struct {
		Email         string `json:"Email"`
		Jwt_for_proof string `json:"Jwt_for_proof"`
	}

	type Response struct {
		Status  string      `json:"status"`
		Data    Data_requst `json:"data,omitempty"`
		Message string      `json:"message"`
	}

	if err == nil && request != nil && len(products) != 0 {
		// тут посылаем код на телефон

		type Kesh struct {
			Passwd []byte
			Code   int
		}

		hash := sha256.Sum256([]byte(passwd))

		// Преобразуем структуру kesh в JSON
		keshData, err := json.Marshal(Kesh{Passwd: hash[:], Code: 777})
		if err != nil {
			log.Fatal("Ошибка при сериализации структуры kesh:", err)
			http.Error(rw, "Ошибка сервера", http.StatusInternalServerError)
			return err
		}

		// Сохраняем код подтверждения в Redis с TTL 10 минут
		err = redisClient.Set(ctx, jwt_for_proof, keshData, 10*time.Minute).Err()
		if err != nil {
			log.Fatal("Ошибка при сохранении кода в Redis:", err)
			http.Error(rw, "Ошибка сервера", http.StatusInternalServerError)
			return err
		}

		response := Response{
			Status:  "success",
			Data:    Data_requst{Email: products[0].Phone_num, Jwt_for_proof: jwt_for_proof},
			Message: "Показано",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)

		return err
	}

	response := Response{
		Status:  "fatal",
		Message: "Не показано",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)

	return err
}

func (repo *MyRepository) EnterCodeForRecoveryPassWithEmailSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, user_id int, passwd string) (err error) {
	request, err := rep.Query(
		ctx,
		`UPDATE users.users SET password_hash = $1 WHERE id = $2 RETURNING id;`,
		passwd,
		user_id)

	errorr(err)

	var id int

	for request.Next() {
		err := request.Scan(
			&id,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}
	}

	type Response struct {
		Status  string `json:"status"`
		Data    int    `json:"data,omitempty"`
		Message string `json:"message"`
	}

	if err != nil || id != 0 {
		response := Response{
			Status:  "success",
			Data:    id,
			Message: "Показано",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)

		return err
	}

	response := Response{
		Status:  "fatal",
		Message: "Не показано",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)

	return err
}

func (repo *MyRepository) EditingLegalUserAvaSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, user_id int, avatar_path string) (err error) {
	request, err := rep.Query(
		ctx,
		`UPDATE users.company_user SET avatar_path = $1 WHERE user_id = $2 RETURNING id;`,
		avatar_path,
		user_id)

	var user_idd int

	for request.Next() {
		err := request.Scan(
			&user_idd,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}
	}

	type Response struct {
		Status  string `json:"status"`
		Data    int    `json:"data,omitempty"`
		Message string `json:"message"`
	}

	if err == nil || user_idd != 0 {
		response := Response{
			Status:  "success",
			Data:    user_idd,
			Message: "Показано",
		}

		json.NewEncoder(rw).Encode(response)

		return err
	}

	response := Response{
		Status:  "fatal",
		Message: "Не показано",
	}

	json.NewEncoder(rw).Encode(response)

	return err
}

func (repo *MyRepository) EditingLegalUserIndNumSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, user_id, ind_num_taxp int) (err error) {
	request, err := rep.Query(
		ctx,
		`UPDATE users.company_user SET ind_num_taxp = $1 WHERE user_id = $2 RETURNING id;`,
		ind_num_taxp,
		user_id)

	var user_idd int

	for request.Next() {
		err := request.Scan(
			&user_idd,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}
	}

	type Response struct {
		Status  string `json:"status"`
		Data    int    `json:"data,omitempty"`
		Message string `json:"message"`
	}

	if err == nil || user_idd != 0 {
		response := Response{
			Status:  "success",
			Data:    user_idd,
			Message: "Показано",
		}

		json.NewEncoder(rw).Encode(response)

		return err
	}

	response := Response{
		Status:  "fatal",
		Message: "Не показано",
	}

	json.NewEncoder(rw).Encode(response)

	return err
}

func (repo *MyRepository) EditingLegalUserNameCompSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, user_id int, name_of_company string) (err error) {
	request, err := rep.Query(
		ctx,
		`UPDATE users.company_user SET name_of_company = $1 WHERE user_id = $2 RETURNING id;`,
		name_of_company,
		user_id)

	var user_idd int

	for request.Next() {
		err := request.Scan(
			&user_idd,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}
	}

	type Response struct {
		Status  string `json:"status"`
		Data    int    `json:"data,omitempty"`
		Message string `json:"message"`
	}

	if err == nil || user_idd != 0 {
		response := Response{
			Status:  "success",
			Data:    user_idd,
			Message: "Показано",
		}

		json.NewEncoder(rw).Encode(response)

		return err
	}

	response := Response{
		Status:  "fatal",
		Message: "Не показано",
	}

	json.NewEncoder(rw).Encode(response)

	return err
}

func (repo *MyRepository) EditingLegalUserAddressNameSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, user_id int, address_name string) (err error) {
	request, err := rep.Query(
		ctx,
		`UPDATE users.company_user SET address_name = $1 WHERE user_id = $2 RETURNING id;`,
		address_name,
		user_id)

	var user_idd int

	for request.Next() {
		err := request.Scan(
			&user_idd,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}
	}

	type Response struct {
		Status  string `json:"status"`
		Data    int    `json:"data,omitempty"`
		Message string `json:"message"`
	}

	if err == nil || user_idd != 0 {
		response := Response{
			Status:  "success",
			Data:    user_idd,
			Message: "Показано",
		}

		json.NewEncoder(rw).Encode(response)

		return err
	}

	response := Response{
		Status:  "fatal",
		Message: "Не показано",
	}

	json.NewEncoder(rw).Encode(response)

	return err
}

func (repo *MyRepository) EditingNaturUserAvaSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, user_id int, avatar_path string) (err error) {
	request, err := rep.Query(
		ctx,
		`UPDATE users.individual_user SET avatar_path = $1 WHERE user_id = $2 RETURNING id;`,
		avatar_path,
		user_id)

	var user_idd int

	for request.Next() {
		err := request.Scan(
			&user_idd,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}
	}

	type Response struct {
		Status  string `json:"status"`
		Data    int    `json:"data,omitempty"`
		Message string `json:"message"`
	}

	if err == nil || user_idd != 0 {
		response := Response{
			Status:  "success",
			Data:    user_idd,
			Message: "Показано",
		}

		json.NewEncoder(rw).Encode(response)

		return err
	}

	response := Response{
		Status:  "fatal",
		Message: "Не показано",
	}

	json.NewEncoder(rw).Encode(response)

	return err
}

func (repo *MyRepository) EditingNaturUserSurnameSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, user_id int, surname string) (err error) {
	request, err := rep.Query(
		ctx,
		`UPDATE users.individual_user SET surname = $1 WHERE user_id = $2 RETURNING id;`,
		surname,
		user_id)

	var user_idd int

	for request.Next() {
		err := request.Scan(
			&user_idd,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}
	}

	type Response struct {
		Status  string `json:"status"`
		Data    int    `json:"data,omitempty"`
		Message string `json:"message"`
	}

	if err == nil || user_idd != 0 {
		response := Response{
			Status:  "success",
			Data:    user_idd,
			Message: "Показано",
		}

		json.NewEncoder(rw).Encode(response)

		return err
	}

	response := Response{
		Status:  "fatal",
		Message: "Не показано",
	}

	json.NewEncoder(rw).Encode(response)

	return err
}

func (repo *MyRepository) EditingNaturUserNameSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, user_id int, name string) (err error) {
	request, err := rep.Query(
		ctx,
		`UPDATE users.individual_user SET name = $1 WHERE user_id = $2 RETURNING id;`,
		name,
		user_id)

	var user_idd int

	for request.Next() {
		err := request.Scan(
			&user_idd,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}
	}

	type Response struct {
		Status  string `json:"status"`
		Data    int    `json:"data,omitempty"`
		Message string `json:"message"`
	}

	if err == nil || user_idd != 0 {
		response := Response{
			Status:  "success",
			Data:    user_idd,
			Message: "Показано",
		}

		json.NewEncoder(rw).Encode(response)

		return err
	}

	response := Response{
		Status:  "fatal",
		Message: "Не показано",
	}

	json.NewEncoder(rw).Encode(response)

	return err
}

func (repo *MyRepository) EditingNaturUserPatronomSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, user_id int, patronymic string) (err error) {
	request, err := rep.Query(
		ctx,
		`UPDATE users.individual_user SET patronymic = $1 WHERE user_id = $2 RETURNING id;`,
		patronymic,
		user_id)

	var user_idd int

	for request.Next() {
		err := request.Scan(
			&user_idd,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}
	}

	type Response struct {
		Status  string `json:"status"`
		Data    int    `json:"data,omitempty"`
		Message string `json:"message"`
	}

	if err == nil || user_idd != 0 {
		response := Response{
			Status:  "success",
			Data:    user_idd,
			Message: "Показано",
		}

		json.NewEncoder(rw).Encode(response)

		return err
	}

	response := Response{
		Status:  "fatal",
		Message: "Не показано",
	}

	json.NewEncoder(rw).Encode(response)

	return err
}

func (repo *MyRepository) EnterCodFromEmailSQL(
	ctx context.Context,
	rw http.ResponseWriter,
	rep *pgxpool.Pool,
	user_id int,
	email string) (err error) {
	request, err := rep.Query(
		ctx,
		`
		UPDATE users.users SET email = $1 WHERE id = $2 RETURNING id;
		`,
		email,
		user_id)

	var user_idd int

	for request.Next() {
		err := request.Scan(
			&user_idd,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}
	}

	type Response struct {
		Status  string `json:"status"`
		Data    int    `json:"data,omitempty"`
		Message string `json:"message"`
	}

	if err == nil || user_idd != 0 {
		response := Response{
			Status:  "success",
			Data:    user_idd,
			Message: "Показано",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)

		return err
	}

	response := Response{
		Status:  "fatal",
		Message: "Не показано",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)

	return err
}

func (repo *MyRepository) EnterCodFromPhoneNumSQL(
	ctx context.Context,
	rw http.ResponseWriter,
	rep *pgxpool.Pool,
	user_id int,
	phone_num string) (err error) {
	request, err := rep.Query(
		ctx,
		`
		UPDATE users.users SET phone_number = $1 WHERE id = $2 RETURNING id;
		`,
		phone_num,
		user_id)

	var user_idd int

	for request.Next() {
		err := request.Scan(
			&user_idd,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}
	}

	type Response struct {
		Status  string `json:"status"`
		Data    int    `json:"data,omitempty"`
		Message string `json:"message"`
	}

	if err == nil || user_idd != 0 {
		response := Response{
			Status:  "success",
			Data:    user_idd,
			Message: "Показано",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)

		return err
	}

	response := Response{
		Status:  "fatal",
		Message: "Не показано",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)

	return err
}

func (repo *MyRepository) AllUserAdsSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, r *http.Request, owner_id int) (err error) {
	var id_list []int
	request, err := rep.Query(
		ctx,
		`
		SELECT array_agg(id) 
		FROM ads.ads 
		WHERE owner_id = $1;
		`,
		owner_id)
	errorr(err)

	for request.Next() {
		err := request.Scan(&id_list)
		if err != nil {
			fmt.Errorf("Error", err)
			continue
		}
	}

	type Response struct {
		Status  string `json:"status"`
		Data    []int  `json:"data,omitempty"`
		Message string `json:"message"`
	}

	if err != nil || len(id_list) == 0 {
		response := Response{
			Status:  "fatal",
			Message: "Не показано",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)

		return err
	}

	response := Response{
		Status:  "success",
		Data:    id_list,
		Message: "Показано",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)
	return
}

func (repo *MyRepository) AllAdsOfThisUserSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, r *http.Request, user_id, owner_id int) (err error) {
	type Product struct {
		Ads_path    string //это фотки объявления
		Avatar_path string //это аватарка юзера

		Ads_photo string
		Avatar    string

		Title         string
		Hourly_rate   int
		Description   string
		Duration      string
		Created_at    time.Time
		Favorite_flag bool
		User_name     string
		Rating        float64
		Review_count  int

		Ads_id      int
		Owner_id    int
		Category_id int
	}

	var Duration_mass []string
	var Favorite_flag_mass []bool
	var Review_count_mass []int

	products := []Product{}
	request, err := rep.Query(
		ctx,
		`
		WITH duration AS (
		SELECT 
			ads.id,
			ads.owner_id,
			ARRAY[
				MAX(bookings.starts_at),
				MAX(bookings.ends_at)
			] AS date_range
		FROM ads.ads
		LEFT JOIN orders.orders 
			ON orders.ad_id = ads.id
		LEFT JOIN orders.bookings 
			ON bookings.order_id = orders.id
		WHERE ads.id = ANY((SELECT array_agg(id) FROM ads.ads WHERE owner_id = $2)::INT[])
		GROUP BY ads.id, ads.owner_id
	)
	SELECT
		COALESCE(t1.File_path::TEXT, '/root/'),
		t2.Title::TEXT,
		t2.Hourly_rate,
		t2.Description::TEXT,
		Duration(
			(SELECT d.date_range::date[] FROM duration d WHERE d.id = t2.id)
		) AS duration_result, -- Функция принимает массив
		t2.Created_at,
		Favorite_flag($1, (SELECT array_agg(id) FROM ads.ads WHERE owner_id = $2)),
		t4.Avatar_path::TEXT as User_avatar,
		COALESCE(t3.Name::TEXT, t5.name_of_company::TEXT) as User_name,
		t4.Rating,
		Review_count((SELECT array_agg(id) FROM ads.ads WHERE owner_id = $2)),
		t2.Id as Ads_id,
		t2.Owner_id,
		t2.Category_id
	FROM
		ads.ads t2
	LEFT JOIN
		ads.ad_photos t1
		ON t2.id = t1.ad_id  -- Соединение на уровне объявления
	LEFT JOIN
		users.individual_user t3
		ON t3.user_id = t2.owner_id
	LEFT JOIN
		users.company_user t5
		ON t5.user_id = t2.owner_id
	LEFT JOIN
		users.users t4
		ON t4.id = t2.owner_id
	WHERE
		t2.status = true
		AND t2.id = ANY((SELECT array_agg(id) FROM ads.ads WHERE owner_id = $2)::INT[]);
		`,
		user_id, owner_id)
	errorr(err)

	for request.Next() {
		p := Product{}
		err := request.Scan(
			&p.Ads_path, //кладем сюда множество, длиною в три ӕлемента, с путями фоток
			&p.Title,
			&p.Hourly_rate,
			&p.Description,
			&Duration_mass,
			&p.Created_at,
			&Favorite_flag_mass,
			&p.Avatar_path,
			&p.User_name,
			&p.Rating,
			&Review_count_mass,

			&p.Ads_id,
			&p.Owner_id,
			&p.Category_id,
		)
		if err != nil {
			fmt.Errorf("Error", err)
			continue
		}
		products = append(products, p)
	}

	for i := 0; i < len(products); i++ { //пока что у нас три объявления
		// products[i].Duration = Duration_mass[i]
		products[i].Favorite_flag = Favorite_flag_mass[i]
		products[i].Review_count = Review_count_mass[i]

		for j := 0; j < len(products[i].Ads_path); j++ {
			products[i].Ads_photo = ServeSpecificMediaBase64(rw, r, products[i].Ads_path)
		}

		products[i].Avatar = ServeSpecificMediaBase64(rw, r, products[i].Avatar_path)
	}

	type Response struct {
		Status  string    `json:"status"`
		Data    []Product `json:"data,omitempty"`
		Message string    `json:"message"`
	}

	if err != nil || len(products) == 0 {
		response := Response{
			Status:  "fatal",
			Message: "Не показано",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)

		return err
	}

	response := Response{
		Status:  "success",
		Data:    products,
		Message: "Показано",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)
	return
}

func (repo *MyRepository) ListOfUserAdsSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, r *http.Request, owner_id int) (err error) {
	type Product struct {
		Ads_path    string //это фотки объявления
		Avatar_path string //это аватарка юзера

		Ads_photo string
		Avatar    string

		Title         string
		Hourly_rate   int
		Daily_rate    int
		Description   string
		Duration      string
		Created_at    time.Time
		Favorite_flag bool
		User_name     string
		Rating        float64
		Review_count  int

		Ads_id      int
		Owner_id    int
		Category_id int
	}

	var Duration_mass []string
	var Favorite_flag_mass []bool
	var Review_count_mass []int

	products := []Product{}
	request, err := rep.Query(
		ctx,
		`
			WITH duration AS (
			  SELECT
				ARRAY_AGG(ads.id) AS ad_ids, -- Собираем все id в массив
				ARRAY_AGG(ARRAY[bookings.starts_at, bookings.ends_at]) AS date_range -- Собираем массив пар дат
			  FROM ads.ads
			  LEFT JOIN orders.bookings 
				ON bookings.ads_id = ads.id
			  LEFT JOIN orders.orders
				ON orders.id = bookings.order_id
			  WHERE owner_id = $1
			)
			SELECT
			  COALESCE(t1.File_path::TEXT, '/root/'),
			  t2.Title::TEXT,
			  t2.Hourly_rate,
			  t2.Daily_rate,
			  t2.Description::TEXT,
			  Duration(
				(SELECT d.date_range::date[] FROM duration d WHERE t2.id = ANY(d.ad_ids))
			  ) AS duration_result, -- Функция принимает массив
			  t2.Created_at,
			  Favorite_flag($1, (SELECT ad_ids FROM duration)::INT[]),
			  t4.Avatar_path::TEXT as User_avatar,
			  COALESCE(t3.Name::TEXT, t5.name_of_company::TEXT) as User_name,
			  t4.Rating,
			  Review_count((SELECT ad_ids FROM duration)::INT[]),
			  t2.Id as Ads_id,
			  t2.Owner_id,
			  t2.Category_id
			FROM
			  ads.ads t2
			LEFT JOIN
			  ads.ad_photos t1
			  ON t2.id = t1.ad_id  -- Соединение на уровне объявления
			LEFT JOIN
			  users.individual_user t3
			  ON t3.user_id = t2.owner_id
			LEFT JOIN
			  users.company_user t5
			  ON t5.user_id = t2.owner_id
			LEFT JOIN
			  users.users t4
			  ON t4.id = t2.owner_id
			WHERE
			  t2.status = true
			  AND t2.id = ANY((SELECT ad_ids FROM duration)::INT[])
			GROUP BY 
			  COALESCE(t1.File_path::TEXT, '/root/'),
			  t2.Title::TEXT,
			  t2.Hourly_rate,
			  t2.Description::TEXT,
			  duration_result, -- Функция принимает массив
			  t2.Created_at,
			  Favorite_flag($1, (SELECT ad_ids FROM duration)::INT[]),
			  User_avatar,
			  User_name,
			  t4.Rating,
			  Review_count((SELECT ad_ids FROM duration)::INT[]),
			  Ads_id,
			  t2.Owner_id,
			  t2.Category_id
			ORDER BY t2.Created_at desc
			`,

		owner_id)
	errorr(err)

	for request.Next() {
		p := Product{}
		err := request.Scan(
			&p.Ads_path, //кладем сюда множество, длиною в три ӕлемента, с путями фоток
			&p.Title,
			&p.Hourly_rate,
			&p.Daily_rate,
			&p.Description,
			&Duration_mass,
			&p.Created_at,
			&Favorite_flag_mass,
			&p.Avatar_path,
			&p.User_name,
			&p.Rating,
			&Review_count_mass,

			&p.Ads_id,
			&p.Owner_id,
			&p.Category_id,
		)
		if err != nil {
			fmt.Errorf("Error", err)
			continue
		}
		products = append(products, p)
	}

	for i := 0; i < len(products); i++ { //пока что у нас три объявления
		// products[i].Duration = Duration_mass[i]
		products[i].Favorite_flag = Favorite_flag_mass[i]
		products[i].Review_count = Review_count_mass[i]

		for j := 0; j < len(products[i].Ads_path); j++ {
			products[i].Ads_photo = ServeSpecificMediaBase64(rw, r, products[i].Ads_path)
		}

		products[i].Avatar = ServeSpecificMediaBase64(rw, r, products[i].Avatar_path)
	}

	type Response struct {
		Status  string    `json:"status"`
		Data    []Product `json:"data,omitempty"`
		Message string    `json:"message"`
	}
	if err != nil || len(products) == 0 {
		response := Response{
			Status:  "fatal",
			Message: "Не показано",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)

		return err
	} else if len(products) == 0 {
		response := Response{
			Status:  "success",
			Data:    []Product{{}},
			Message: "Показано",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)

		return
	}

	response := Response{
		Status:  "success",
		Data:    products,
		Message: "Показано",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)
	return
}

func (repo *MyRepository) RecoveryPasswdEmailSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, r *http.Request, redisClient *redis.Client, email string) (err error) {
	type Data struct {
		Email_name     string `json:"Email_name"`
		ValidToken_jwt string `json:"ValidToken_jwt"`
	}

	type Response struct {
		Status  string `json:"status"`
		Data    Data   `json:"data,omitempty"`
		Message string `json:"message"`
	}

	var id int
	request := rep.QueryRow(
		ctx, `
		SELECT users.id
		FROM Users.users
			LEFT JOIN Users.individual_user ON users.id = individual_user.user_id
			LEFT JOIN Users.company_user ON users.id = company_user.user_id
		WHERE email = $1;`,
		email)
	err = request.Scan(&id)
	errorr(err)

	if id == 0 {
		response := Response{
			Status:  "fatal",
			Message: "Логине не принят",
		}

		json.NewEncoder(rw).Encode(response)

		return
	}

	// Настройки SMTP-сервера
	smtpHost := "smtp.mail.ru"
	smtpPort := "587"

	// Данные отправителя (ваша почта и пароль приложения)
	senderEmail := "parpatt_test@mail.ru"
	password := "X0h72ndPXchhjWZ4vbyT" // Пароль приложения

	// Получатель
	recipientEmail := email

	// Сообщение
	subject := "Subject: Тебя беспокоит служба безопасности сбербанка.\n"
	body := "Введи этот код.\n"
	// Генерация случайного кода
	// Диапазон четырёхзначных чисел: от 1000 до 9999
	min, max := 1000, 9999
	// Вычисляем размер диапазона
	rangeSize := big.NewInt(int64(max - min + 1))
	// Генерируем случайное число в диапазоне от 0 до rangeSize-1
	n, err := rand.Int(rand.Reader, rangeSize)
	if err != nil {
		// logger.Err(err).Msg(fmt.Sprintf("error in %s", op+"; Ошибка генерации случайного числа"))

		return
	}
	// Смещаем результат, чтобы получить число в диапазоне от min до max
	codeNum := int(n.Int64() + int64(min)) // искомое число

	message := []byte(subject + "\n" + body + strconv.Itoa(codeNum))

	// Авторизация для отправки email
	auth := smtp.PlainAuth("", senderEmail, password, smtpHost)

	// Устанавливаем обычное нешифрованное соединение
	client, err := smtp.Dial(smtpHost + ":" + smtpPort)
	if err != nil {
		log.Fatal(err)
	}

	// Используем команду STARTTLS для начала TLS-сессии
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true, // Это нужно убрать в продакшене
		ServerName:         smtpHost,
	}

	if err = client.StartTLS(tlsConfig); err != nil {
		log.Fatal(err)
	}

	// Старт авторизации
	if err = client.Auth(auth); err != nil {
		log.Fatal(err)
	}

	// Установка адреса отправителя
	if err = client.Mail(senderEmail); err != nil {
		log.Fatal(err)
	}

	// Установка адреса получателя
	if err = client.Rcpt(recipientEmail); err != nil {
		log.Fatal(err)
	}

	// Отправка сообщения
	w, err := client.Data()
	if err != nil {
		log.Fatal(err)
	}

	_, err = w.Write(message)
	if err != nil {
		log.Fatal(err)
	}

	err = w.Close()
	if err != nil {
		log.Fatal(err)
	}

	// Завершение сеанса
	client.Quit()

	// Генерация JWT токена
	ValidToken_jwt, err := jwt.GenerateJWT("jwt_for_proof", 0, 0)
	errorr(err)

	// Установка куки
	livingTime := 10 * time.Minute //не смог найти чего-то получше
	expiration := time.Now().Add(livingTime)
	cookie := http.Cookie{
		Name:     "token",
		Value:    url.QueryEscape(ValidToken_jwt),
		Expires:  expiration,
		Path:     "/",             // Убедитесь, что путь корректен
		Domain:   "185.112.83.36", // IP-адрес вашего сервера
		HttpOnly: true,
		Secure:   false, // Для HTTP можно оставить false
		SameSite: http.SameSiteLaxMode,
	}

	_ = cookie

	type Kesh struct {
		CodeNum string
		Login   string
	}

	// Преобразуем структуру kesh в JSON
	keshData, err := json.Marshal(Kesh{CodeNum: strconv.Itoa(codeNum), Login: email})
	errorr(err)

	// Сохраняем код подтверждения в Redis с TTL 10 минут
	err = redisClient.Set(ctx, ValidToken_jwt, keshData, 10*time.Minute).Err()
	errorr(err)

	if err != nil || ValidToken_jwt == "" {
		response := Response{
			Status:  "fatal",
			Message: "Почта не принята",
		}

		json.NewEncoder(rw).Encode(response)

		return
	}

	response := Response{
		Status:  "success",
		Data:    Data{email, ValidToken_jwt},
		Message: "Почта принята",
	}

	json.NewEncoder(rw).Encode(response)

	return
}

func (h *SignupHandler) RecoveryPasswdPhoneSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, r *http.Request, redisClient *redis.Client, phone_num string) (err error) {
	type Data struct {
		Phone_num      string `json:"Phone_num"`
		ValidToken_jwt string `json:"ValidToken_jwt"`
	}

	type Response struct {
		Status  string `json:"status"`
		Data    Data   `json:"data,omitempty"`
		Message string `json:"message"`
	}

	var id int
	request := rep.QueryRow(
		ctx, `
		SELECT users.id
		FROM Users.users
			LEFT JOIN Users.individual_user ON users.id = individual_user.user_id
			LEFT JOIN Users.company_user ON users.id = company_user.user_id
		WHERE phone_number = $1;`,
		phone_num)
	err = request.Scan(&id)
	errorr(err)

	if id == 0 {
		response := Response{
			Status:  "fatal",
			Message: "Логине не принят",
		}

		json.NewEncoder(rw).Encode(response)

		return
	}

	type Kesh struct {
		CodeNum string
		Login   string
	}

	// Генерация случайного кода
	// Диапазон четырёхзначных чисел: от 1000 до 9999
	min, max := 1000, 9999
	// Вычисляем размер диапазона
	rangeSize := big.NewInt(int64(max - min + 1))
	// Генерируем случайное число в диапазоне от 0 до rangeSize-1
	n, err := rand.Int(rand.Reader, rangeSize)
	if err != nil {
		// logger.Err(err).Msg(fmt.Sprintf("error in %s", op+"; Ошибка генерации случайного числа"))

		return
	}
	// Смещаем результат, чтобы получить число в диапазоне от min до max
	h.CodeNum = int(n.Int64() + int64(min)) // искомое число

	// Преобразуем структуру kesh в JSON
	keshData, err := json.Marshal(Kesh{CodeNum: strconv.Itoa(h.CodeNum), Login: phone_num})
	if err != nil {
		// logger.Err(err).Msg(" error in internal.services.user.SignupUserByPhone; ошибка с структурой json или преобразованием типа")

		return
	}

	request_url := "https://zvonok.com/manager/cabapi_external/api/v1/phones/flashcall/"

	// Создание буфера для тела запроса
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	// Добавление полей в multipart-запрос
	writer.WriteField("public_key", "ba885d6d0342490a50c6bf5603d75719")
	writer.WriteField("phone", phone_num)
	writer.WriteField("campaign_id", "1771893356")
	writer.WriteField("phone_suffix", strconv.Itoa(h.CodeNum))

	// Закрытие writer (важно!)
	writer.Close()

	// Создание HTTP-запроса
	req, err := http.NewRequest("POST", request_url, &requestBody)
	if err != nil {
		fmt.Println("Ошибка при создании запроса:", err)
		os.Exit(1)
	}

	// Установка заголовка Content-Type
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Отправка запроса
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Ошибка при выполнении запроса:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	// Чтение ответа
	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	fmt.Println("Ответ от сервера:", buf.String())

	// Генерация JWT токена
	ValidToken_jwt, err := jwt.GenerateJWT("jwt_for_proof", 0, 0)
	errorr(err)

	// Установка куки
	livingTime := 10 * time.Minute //не смог найти чего-то получше
	expiration := time.Now().Add(livingTime)
	cookie := http.Cookie{
		Name:     "token",
		Value:    url.QueryEscape(ValidToken_jwt),
		Expires:  expiration,
		Path:     "/",             // Убедитесь, что путь корректен
		Domain:   "185.112.83.36", // IP-адрес вашего сервера
		HttpOnly: true,
		Secure:   false, // Для HTTP можно оставить false
		SameSite: http.SameSiteLaxMode,
	}

	_ = cookie

	// Сохраняем код подтверждения в Redis с TTL 10 минут
	err = redisClient.Set(ctx, ValidToken_jwt, keshData, 10*time.Minute).Err()
	errorr(err)

	if err != nil || ValidToken_jwt == "" {
		response := Response{
			Status:  "fatal",
			Message: "Почта не принята",
		}

		json.NewEncoder(rw).Encode(response)

		return
	}

	response := Response{
		Status:  "success",
		Data:    Data{phone_num, ValidToken_jwt},
		Message: "Почта принята",
	}

	json.NewEncoder(rw).Encode(response)

	return
}

func (repo *MyRepository) RecoveryPasswdCodeSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, redisClient *redis.Client, passwd string, code string, login string) (err error) {
	request, err := rep.Query(
		ctx,
		`UPDATE users.users SET password_hash = $1 WHERE email = $2 OR phone_number = $2 RETURNING id;`,
		passwd,
		login)

	errorr(err)

	var id int

	for request.Next() {
		err := request.Scan(
			&id,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}
	}

	type Response struct {
		Status  string `json:"status"`
		Data    int    `json:"data,omitempty"`
		Message string `json:"message"`
	}

	if err != nil || id != 0 {
		response := Response{
			Status:  "success",
			Data:    id,
			Message: "Показано",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)

		return err
	}

	response := Response{
		Status:  "fatal",
		Message: "Не показано",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)

	return err
}

func (repo *MyRepository) SendCodeSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, r *http.Request, redisClient *redis.Client, token *http.Cookie, code int) (err error) {
	// Получаем данные из Redis
	keshData, err := redisClient.Get(ctx, token.Value).Result()
	if err == redis.Nil {
		log.Println("Ключ не найден")
		http.Error(rw, "Код не найден или истек", http.StatusUnauthorized)
		return
	} else if err != nil {
		log.Fatal("Ошибка при получении данных из Redis:", err)
		http.Error(rw, "Ошибка сервера", http.StatusInternalServerError)
		return
	}

	type Kesh struct {
		CodeNum    int
		Email_name string
	}

	// Десериализуем JSON обратно в структуру kesh
	var storedKesh Kesh
	err = json.Unmarshal([]byte(keshData), &storedKesh)
	if err != nil {
		log.Fatal("Ошибка при десериализации данных из Redis:", err)
		http.Error(rw, "Ошибка сервера", http.StatusInternalServerError)
		return
	}

	// Проверяем код
	if code == storedKesh.CodeNum && code != 0 && storedKesh.CodeNum != 0 {
		type Data struct {
			ValidToken_jwt string `json:"ValidToken_jwt"`
		}

		type Response struct {
			Status  string `json:"status"`
			Data    Data   `json:"data,omitempty"`
			Message string `json:"message"`
		}

		// Генерация JWT токена
		ValidToken_jwt, err := jwt.GenerateJWT("jwt_for_proof", 0, 0)
		errorr(err)

		fmt.Println("JWT токен в строковом формате: ", ValidToken_jwt)

		// Установка куки
		livingTime := 10 * time.Minute //не смог найти чего-то получше
		expiration := time.Now().Add(livingTime)
		cookie := http.Cookie{
			Name:     "token",
			Value:    url.QueryEscape(ValidToken_jwt),
			Expires:  expiration,
			Path:     "/",             // Убедитесь, что путь корректен
			Domain:   "185.112.83.36", // IP-адрес вашего сервера
			HttpOnly: true,
			Secure:   false, // Для HTTP можно оставить false
			SameSite: http.SameSiteLaxMode,
		}

		fmt.Printf("Кука установлена: %v\n", cookie)

		CodeNum := 777 // Здесь лучше использовать случайный код

		type Kesh struct {
			CodeNum    int
			Email_name string
		}

		// Преобразуем структуру kesh в JSON
		keshData, err := json.Marshal(Kesh{CodeNum: CodeNum, Email_name: storedKesh.Email_name})
		errorr(err)

		// Сохраняем код подтверждения в Redis с TTL 10 минут
		err = redisClient.Set(ctx, ValidToken_jwt, keshData, 10*time.Minute).Err()
		errorr(err)

		rw.WriteHeader(http.StatusOK)

		if err != nil || ValidToken_jwt == "" {
			response := Response{
				Status:  "fatal",
				Message: "Почта не принята",
			}

			json.NewEncoder(rw).Encode(response)

			return err
		}

		response := Response{
			Status:  "success",
			Data:    Data{ValidToken_jwt},
			Message: "Почта принята",
		}

		json.NewEncoder(rw).Encode(response)

		if err != nil {
			log.Fatal(err)
		}
		rw.Write([]byte("Смена завершена успешно"))
		return err

	} else {
		http.Error(rw, "Неверный код подтверждения", http.StatusUnauthorized)
		return err

	}
}

func (repo *MyRepository) EnterPasswdSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, r *http.Request, redisClient *redis.Client, token *http.Cookie, passwd string) (err error) {
	// Получаем данные из Redis
	keshData, err := redisClient.Get(ctx, token.Value).Result()
	if err == redis.Nil {
		log.Println("Ключ не найден")
		http.Error(rw, "Код не найден или истек", http.StatusUnauthorized)
		return
	} else if err != nil {
		log.Fatal("Ошибка при получении данных из Redis:", err)
		http.Error(rw, "Ошибка сервера", http.StatusInternalServerError)
		return
	}

	type Kesh struct {
		CodeNum    int
		Email_name string
	}

	// Десериализуем JSON обратно в структуру kesh
	var storedKesh Kesh
	err = json.Unmarshal([]byte(keshData), &storedKesh)
	if err != nil {
		log.Fatal("Ошибка при десериализации данных из Redis:", err)
		http.Error(rw, "Ошибка сервера", http.StatusInternalServerError)
		return
	}

	request, err := rep.Query(
		ctx,
		`
		UPDATE users.users SET password_hash = $1 WHERE email = $2 OR phone_number = $2 RETURNING id;
		`,
		passwd,
		storedKesh.Email_name)

	var user_idd int

	for request.Next() {
		err := request.Scan(
			&user_idd,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}
	}

	type Response struct {
		Status  string `json:"status"`
		Data    int    `json:"data,omitempty"`
		Message string `json:"message"`
	}

	if err == nil || user_idd != 0 {
		response := Response{
			Status:  "success",
			Data:    user_idd,
			Message: "Показано",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)

		return err
	}

	response := Response{
		Status:  "fatal",
		Message: "Не показано",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)

	return err
}

func (repo *MyRepository) AddAddressSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, r *http.Request, redisClient *redis.Client, user_id int, adress string) (err error) {

	type Response struct {
		Status  string
		Data    int
		Message string
	}
	result, errors := rep.Query(ctx, `
		INSERT INTO Users.address (user_id, address_name)
		VALUES ($1, $2)
		RETURNING user_id;
		`,
		user_id,
		adress,
	)
	errorr(err)

	var User_id int
	for result.Next() {
		err := result.Scan(
			&User_id,
		)
		fmt.Println(err)

		if err != nil {
			fmt.Println(err)
			continue
		}
	}

	if User_id == 0 || errors != nil {
		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(Response{
			Status:  "fatal",
			Message: "Введите корректный и уникальный адрес ",
		})

		return
	} else if errors != nil {
		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(Response{
			Status:  "fatal",
			Message: errors.Error(),
		})

		return
	} else {
		response := Response{
			Status:  "success",
			Data:    User_id,
			Message: "Адрес добавлен",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)

		return
	}
}

func (repo *MyRepository) GiveAddressSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, r *http.Request, redisClient *redis.Client, user_id int) (err error) {

	type Response struct {
		Status  string
		Data    []string
		Message string
	}
	result, errors := rep.Query(ctx, `
			SELECT address_name FROM Users.address WHERE user_id = $1
		`,
		user_id,
	)
	errorr(err)

	var address string
	address_mass := []string{}

	for result.Next() {
		err := result.Scan(
			&address,
		)
		fmt.Println(err)

		address_mass = append(address_mass, address)

		if err != nil {
			fmt.Println(err)
			continue
		}
	}

	if address == "" || errors != nil {
		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(Response{
			Status:  "fatal",
			Message: "Введите корректный и уникальный адрес ",
		})

		return
	} else if errors != nil {
		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(Response{
			Status:  "fatal",
			Message: errors.Error(),
		})

		return
	} else {
		response := Response{
			Status:  "success",
			Data:    address_mass,
			Message: "Адрес добавлен",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)

		return
	}
}
