package user

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"mime/multipart"
	"os"

	function "myproject/internal"
	"myproject/internal/database"
	"myproject/internal/jwt"
	"myproject/internal/model"
	"net"
	"net/http"
	"net/smtp"
	"net/url"

	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog"
)

type SignupHandler struct {
	RedisClient  *redis.Client
	Logger       zerolog.Logger
	EmailOrPhone string
	CodeNum      int
	RespError    error
	JWT          string
	RequestURL   string
}

func verifyEmail(email string) bool {
	// Разбиваем email на части
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false // Неверный формат email
	}

	// Получаем домен
	domain := parts[1]
	// Проверяем наличие MX-записей у домена
	mxRecords, err := net.LookupMX(domain)
	if err != nil || len(mxRecords) == 0 {
		return false // Домен не существует или нет почтового сервера
	}

	server := mxRecords[0].Host
	client, err := smtp.Dial(server + ":25")
	if err != nil {
		return false
	}
	defer client.Close()

	if err = client.Mail("test@example.com"); err != nil {
		return false
	}
	if err = client.Rcpt(email); err != nil {
		return false
	}

	return true
}

func SignupUserByEmailCreater(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context) httprouter.Handle {
	op := "internal.services.user.SignupUserByEmailCreater"

	// Генерация случайного кода
	// Диапазон четырёхзначных чисел: от 1000 до 9999
	min, max := 1000, 9999
	// Вычисляем размер диапазона
	rangeSize := big.NewInt(int64(max - min + 1))
	// Генерируем случайное число в диапазоне от 0 до rangeSize-1
	n, err := rand.Int(rand.Reader, rangeSize)
	if err != nil {
		logger.Err(err).Msg(fmt.Sprintf("error in %s; Ошибка генерации случайного числа", op))

		return nil
	}

	// Генерация JWT токена
	ValidToken_jwt, err := jwt.GenerateJWT("jwt_for_reg", 0, 0)
	if err != nil {
		logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка при генерации JWT", op))

		return nil
	}

	// Смещаем результат, чтобы получить число в диапазоне от min до max
	handler := &SignupHandler{
		RedisClient: redisClient,
		Logger:      logger,
		CodeNum:     int(n.Int64() + int64(min)),
		JWT:         ValidToken_jwt,
	}

	return handler.SignupUserByEmail(logger, ctx)
}

func (h *SignupHandler) SignupUserByEmail(logger zerolog.Logger, ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.user.SignupUserByEmail"

		type Kesh struct {
			Email string `json:"Email"`
			Code  int    `json:"Code"`
		}

		type Data struct {
			ValidToken_jwt string `json:"ValidToken_jwt"`
		}

		// w.WriteHeader(http.StatusOK)
		type Response struct {
			Status  string `json:"status"`
			Data    string `json:"data,omitempty"`
			Message string `json:"message"`
		}

		var kesh Kesh

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&kesh)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка при парсинге JSON-запроса", op))
		}

		// Настройки SMTP-сервера
		smtpHost := "smtp.mail.ru"
		smtpPort := "587"

		// Данные отправителя (ваша почта и пароль приложения)
		senderEmail := "parpatt_test@mail.ru"
		password := "X0h72ndPXchhjWZ4vbyT" // Пароль приложения

		// Получатель
		recipientEmail := kesh.Email

		if !verifyEmail(recipientEmail) {
			err := fmt.Errorf("почты %s не существует", recipientEmail)
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с почтой получателя: %v", op, err))

			response := Response{
				Status:  "fatal",
				Message: "Почты получателя не существует",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		// Формирование сообщения
		message := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: Ваш код подтверждения\r\n\r\nВаш код подтверждения: %d",
			senderEmail, recipientEmail, h.CodeNum))

		// Авторизация SMTP
		auth := smtp.PlainAuth("", senderEmail, password, smtpHost)

		// Отправка письма
		err = smtp.SendMail(
			smtpHost+":"+smtpPort,
			auth,
			senderEmail,
			[]string{recipientEmail},
			message,
		)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка при создании интерфейса для записи", op))

			return
		}

		// Сохранение кода в Redis
		keshData, err := json.Marshal(Kesh{Email: kesh.Email, Code: h.CodeNum})
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка сериализации JSON", op))

			return
		}
		livingTime := 40 * time.Minute

		err = h.RedisClient.Set(ctx, h.JWT, string(keshData), livingTime).Err()
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s", op+"; Ошибка при сохранении кода в Redis"))
			response := Response{
				Status:  "fatal",
				Message: "Почта не принята",
			}

			// Лог с контекстом
			logger.Info().
				Str("service", "login").
				Int("port", 8080).
				Msg("User enter code and email")

			json.NewEncoder(w).Encode(response)

			return
		}

		w.WriteHeader(http.StatusOK)

		if h.JWT == "" {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; Ошибка, JWT пуст", op))
		}

		response := Response{
			Status:  "success",
			Data:    h.JWT,
			Message: "Почта принята",
		}

		json.NewEncoder(w).Encode(response)

		return
	}
}

type RealFlashCallClient struct {
	PublicKey  string
	CampaignID string
	BaseURL    string // https://zvonok.com/manager/cabapi_external/api/v1/phones/flashcall/
	HTTPClient *http.Client
}

func SignupUserByPhoneCreater(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context) httprouter.Handle {
	op := "internal.services.user.SignupUserByEmailCreater"

	// Генерация случайного кода
	// Диапазон четырёхзначных чисел: от 1000 до 9999
	min, max := 1000, 9999
	// Вычисляем размер диапазона
	rangeSize := big.NewInt(int64(max - min + 1))
	// Генерируем случайное число в диапазоне от 0 до rangeSize-1
	n, err := rand.Int(rand.Reader, rangeSize)
	if err != nil {
		logger.Err(err).Msg(fmt.Sprintf("error in %s; Ошибка генерации случайного числа", op))

		return nil
	}

	// Генерация JWT токена
	ValidToken_jwt, err := jwt.GenerateJWT("jwt_for_reg", 0, 0)
	if err != nil {
		logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка при генерации JWT", op))

		return nil
	}

	// Смещаем результат, чтобы получить число в диапазоне от min до max
	handler := &SignupHandler{
		RedisClient: redisClient,
		Logger:      logger,
		CodeNum:     int(n.Int64() + int64(min)),
		JWT:         ValidToken_jwt,
		RequestURL:  "https://zvonok.com/manager/cabapi_external/api/v1/phones/flashcall/",
	}

	return handler.SignupUserByPhone(logger, ctx)
}

func (h *SignupHandler) SignupUserByPhone(logger zerolog.Logger, ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.user.SignupUserByPhone"

		type Kesh struct {
			Phone_number string `json:"Phone_num"`
			Code         int    `json:"Code"`
		}

		var kesh Kesh

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&kesh)
		if err != nil {
			logger.Err(err).Msg("error in internal.services.user.SignupUserByPhone; ошибка при парсинге JSON-запроса")

			return
		}

		// Преобразуем структуру kesh в JSON
		keshData, err := json.Marshal(Kesh{Phone_number: kesh.Phone_number, Code: h.CodeNum})
		if err != nil {
			logger.Err(err).Msg("error in internal.services.user.SignupUserByPhone; ошибка с структурой json или преобразованием типа")

			return
		}

		// Создание буфера для тела запроса
		var requestBody bytes.Buffer
		writer := multipart.NewWriter(&requestBody)

		// Добавление полей в multipart-запрос
		writer.WriteField("public_key", "ba885d6d0342490a50c6bf5603d75719")
		writer.WriteField("phone", kesh.Phone_number)
		writer.WriteField("campaign_id", "1771893356")
		writer.WriteField("phone_suffix", strconv.Itoa(h.CodeNum))

		// Закрытие writer (важно!)
		defer writer.Close()

		// Создание HTTP-запроса
		req, err := http.NewRequest("POST", h.RequestURL, &requestBody)
		if err != nil {
			logger.Err(err).Msg("Ошибка при создании запроса")

			os.Exit(1)
		}

		// Установка заголовка Content-Type
		req.Header.Set("Content-Type", writer.FormDataContentType())

		// Отправка запроса
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			logger.Err(err).Msg("Ошибка при выполнении запроса")

			os.Exit(1)
		}
		defer resp.Body.Close()

		// Чтение ответа
		buf := new(bytes.Buffer)
		buf.ReadFrom(resp.Body)

		// Установка куки
		livingTime := 40 * time.Minute //не смог найти чего-то получше
		expiration := time.Now().Add(livingTime)
		cookie := http.Cookie{
			Name:     "token",
			Value:    url.QueryEscape(h.JWT),
			Expires:  expiration,
			Path:     "/",             // Убедитесь, что путь корректен
			Domain:   "185.112.83.36", // IP-адрес вашего сервера
			HttpOnly: true,
			Secure:   false, // Для HTTP можно оставить false
			SameSite: http.SameSiteLaxMode,
		}

		_ = cookie

		// Сохраняем код подтверждения в Redis с TTL 40 минут
		err = h.RedisClient.Set(ctx, h.JWT, string(keshData), livingTime).Err()
		if err != nil {
			logger.Err(err).Msg(" error in internal.services.user.SignupUserByPhone; Ошибка при сохранении кода в Redis")

			return
		}

		type Data struct {
			ValidToken_jwt string `json:"ValidToken_jwt"`
		}

		w.WriteHeader(http.StatusOK)
		type Response struct {
			Status  string `json:"status"`
			Data    Data   `json:"data,omitempty"`
			Message string `json:"message"`
		}

		if h.JWT == "" {
			logger.Info().Msg("Телефон не принят " + op + " Номер телефона: " + kesh.Phone_number)

			response := Response{
				Status:  "fatal",
				Message: "Телефон не принят",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		response := Response{
			// logger.Info().Msg("Телефон принята " + op + " Номер телефона: " + kesh.Phone_number)

			Status:  "success",
			Data:    Data{h.JWT},
			Message: "Телефон принят",
		}

		json.NewEncoder(w).Encode(response)
	}
}

func EnterCodeFromEmailCreater(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		op := "internal.services.user.EnterCodeFromEmailCreater"

		// Попытка прочитать куку
		token, err := r.Cookie("request_token")
		if err != nil {
			if err == http.ErrNoCookie {
				logger.Err(err).Msg(" error in " + op + "; Кука не найдена")

				return
			}

			logger.Err(err).Msg(" error in " + op + "; Проблема с чтением куки")
			return

		}

		if token.Value == "" {
			logger.Err(err).Msg(" error in " + op + "; Токен пуст")

			return

		}

		token_flag, _, _ := jwt.IsAuthorized(token.Value)
		if !token_flag {
			logger.Err(err).Msg(" error in " + op + "; Неверный токен")

			return

		}

		handler := &SignupHandler{
			RedisClient: redisClient,
			Logger:      logger,
			JWT:         token.Value,
		}

		// 4) Вызываем дальше нужную логику: например, метод EnterCodeFromEmail
		subHandler := handler.EnterCodeFromEmail(ctx)
		subHandler(w, r, ps) // передаём управление вашему вторичному хендлеру
	}
}

func (h *SignupHandler) EnterCodeFromEmail(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.user.EnterCodeFromEmail"
		type RegCode struct {
			Email string `json:"Email"`
			Code  int    `json:"Code"`
		}

		var email RegCode

		// Создаем структуру ответа
		type Response struct {
			Status  string `json:"status"`
			Data    string `json:"data,omitempty"`
			Message string `json:"message"`
		}

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&email)
		if err != nil {
			h.Logger.Err(err).Msg(" error in " + op + "; Ошибка при сохранении кода в парсинге JSON-запроса")

			h.Logger.Info().
				Str("service", op).
				Int("port", 8080).
				Msg("Неверный формат кода")

			response := Response{
				Status:  "fatal",
				Message: "Неверный формат кода",
			}
			// Отправляем ответ
			err = json.NewEncoder(w).Encode(response)
			if err != nil {
				h.Logger.Err(err).Msg(" error in " + op + "; Ошибка при отправке ответа после операции")

				return
			}

			return
		}

		// Получаем данные из Redis
		keshData, err := h.RedisClient.Get(ctx, h.JWT).Result()
		if err == redis.Nil {
			h.Logger.Err(err).Msg(" error in " + op + "; Ключ не найден")

			return
		} else if err != nil {
			h.Logger.Err(err).Msg(" error in " + op + "; Ошибка при получении данных из Redis")

			return
		}

		// Десериализуем JSON обратно в структуру kesh
		var storedKesh RegCode
		err = json.Unmarshal([]byte(keshData), &storedKesh)
		if err != nil {
			h.Logger.Err(err).Msg(" error in " + op + "; Ошибка при десериализации данных из Redis")

			return
		}

		var response Response

		// Проверяем код
		if email.Code == storedKesh.Code {
			h.Logger.Info().
				Str("service", op).
				Int("port", 8080).
				Msg("Код принят")

			response = Response{
				Status:  "success",
				Data:    storedKesh.Email,
				Message: "Код принят",
			}
		} else {
			h.Logger.Info().
				Str("service", op).
				Int("port", 8080).
				Msg("Неверный код")

			response = Response{
				Status:  "fatal",
				Message: "Неверный код",
			}
		}

		// Отправляем ответ
		err = json.NewEncoder(w).Encode(response)
		if err != nil {
			h.Logger.Err(err).Msg(" error in " + op + "; Ошибка при отправке ответа после операции")

			return
		}
		return
	}
}

func EnterCodeFromPhoneCreater(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		op := "internal.services.user.EnterCodeFromEmailCreater"

		// Попытка прочитать куку
		token, err := r.Cookie("request_token")
		if err != nil {
			if err == http.ErrNoCookie {
				logger.Err(err).Msg(" error in " + op + "; Кука не найдена")

				return
			}

			logger.Err(err).Msg(" error in " + op + "; Проблема с чтением куки")
			return

		}

		if token.Value == "" {
			logger.Err(err).Msg(" error in " + op + "; Токен пуст")

			return

		}

		token_flag, _, _ := jwt.IsAuthorized(token.Value)
		if !token_flag {
			logger.Err(err).Msg(" error in " + op + "; Неверный токен")

			return

		}

		handler := &SignupHandler{
			RedisClient: redisClient,
			Logger:      logger,
			JWT:         token.Value,
		}

		// 4) Вызываем дальше нужную логику: например, метод EnterCodeFromEmail
		subHandler := handler.EnterCodeFromPhone(ctx)
		subHandler(w, r, ps) // передаём управление вашему вторичному хендлеру
	}
}

func (h *SignupHandler) EnterCodeFromPhone(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.user.EnterCodeFromPhone"
		var phone model.Reg_code

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&phone)
		if err != nil {

			return
		}

		// Получаем данные из Redis
		keshData, err := h.RedisClient.Get(ctx, h.JWT).Result()
		if err == redis.Nil {
			h.Logger.Err(err).Msg(fmt.Sprintf("error in %v; Ключ не найден", op))

			return
		} else if err != nil {
			h.Logger.Err(err).Msg(" error in " + op + "; Ошибка при получении данных из Redis")

			return
		}

		// Десериализуем JSON обратно в структуру kesh
		var storedKesh model.Phone_kesh
		err = json.Unmarshal([]byte(keshData), &storedKesh)
		if err != nil {
			h.Logger.Err(err).Msg(" error in " + op + "; Ошибка при десериализации данных из Redis")

			h.Logger.Err(err).Msg(" error in " + op + "; Ошибка при сохранении кода в парсинге JSON-запроса")

			h.Logger.Info().
				Str("service", op).
				Int("port", 8080).
				Msg("Неверный формат кода")

			response := model.Response{
				Status:  "fatal",
				Message: "Неверный формат кода",
			}
			// Отправляем ответ
			err = json.NewEncoder(w).Encode(response)
			if err != nil {
				h.Logger.Err(err).Msg(" error in " + op + "; Ошибка при отправке ответа после операции")

				return
			}

			return
		}
		response := model.Response{}

		// Проверяем код
		if phone.Reg_code == storedKesh.Code {
			response = model.Response{
				Status:  "success",
				Data:    storedKesh.Phone,
				Message: "Код принят",
			}

			w.WriteHeader(http.StatusOK)
		} else {
			response = model.Response{
				Status:  "fatal",
				Message: "Неверный код",
			}
			w.WriteHeader(http.StatusUnauthorized) // Для неверного кода лучше использовать статус 401 Unauthorized
		}

		// Отправляем ответ
		err = json.NewEncoder(w).Encode(response)
		if err != nil {
			h.Logger.Err(err).Msg(" error in " + op + "; Ошибка при отправке ответа")

			return
		}
		return
	}
}

func SignupLegalEmailCreater(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		op := "internal.services.user.EnterCodeFromEmailCreater"

		// Попытка прочитать куку
		token, err := r.Cookie("token")
		if err != nil {
			if err == http.ErrNoCookie {
				logger.Err(err).Msg(" error in " + op + "; Кука не найдена")

				return
			}

			logger.Err(err).Msg(" error in " + op + "; Проблема с чтением куки")

			return
		}

		if token.Value == "" {
			logger.Err(err).Msg(" error in " + op + "; Токен пуст")

			return
		}

		token_flag, _, _ := jwt.IsAuthorized(token.Value)
		if !token_flag {
			logger.Err(err).Msg(" error in " + op + "; Неверный токен")

			return
		}

		handler := &SignupHandler{
			RedisClient: redisClient,
			Logger:      logger,
			JWT:         token.Value,
		}

		// 4) Вызываем дальше нужную логику: например, метод EnterCodeFromEmail
		subHandler := handler.SignupLegalEmail(ctx, dbpool)
		subHandler(w, r, ps) // передаём управление вашему вторичному хендлеру
	}
}

func (h *SignupHandler) SignupLegalEmail(ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.user.SignupLegalEmail"
		var user model.LegalUser

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&user)
		if err != nil {
			h.Logger.Err(err).Msg(" error in " + op + "; Ошибка при отправке ответа")

			return
		}

		flag := function.ValidatePassword(w, user.Password_hash)

		if !flag {
			h.Logger.Err(err).Msg(" error in " + op + "; пароль не валиден")

			return
		}

		// Получаем данные из Redis
		keshData, err := h.RedisClient.Get(ctx, h.JWT).Result()
		if err == redis.Nil {
			h.Logger.Err(err).Msg(" error in " + op + "; Код не найден или истек")

			return
		} else if err != nil {
			h.Logger.Err(err).Msg(" error in " + op + "; Ошибка при получении данных из Redis")

			return
		}

		// Десериализуем JSON обратно в структуру kesh
		var storedKesh model.Email_kesh
		err = json.Unmarshal([]byte(keshData), &storedKesh)
		if err != nil {
			h.Logger.Err(err).Msg(" error in " + op + "; Ошибка при десериализации данных из Redis")

			return
		}

		type Response struct {
			Status  string `json:"status"`
			Data    int    `json:"data"`
			Message string `json:"message"`
		}

		if storedKesh.Email == "" || user.Ind_num_taxp == 0 || user.Name_of_company == "" || user.Address_name == "" {
			response := Response{
				Status:  "fatal",
				Data:    0,
				Message: "Передаёшь пустое значение",
			}

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)

			return
		}

		repo := database.NewRepo(ctx, dbpool)

		var pwd = "/home/beeline/media/user/"

		fmt.Println("Email: ", storedKesh.Email)

		err, image_flag, file_path := function.UploadAvatar(w, user.Avatar, pwd, strings.Split(storedKesh.Email, ".")[0], "ava")
		if err != nil {
			h.Logger.Err(err).Msg(" error in " + op + "; Ошибка при добавлении аватарки")

			return
		}

		if image_flag {
			err = repo.SigLegalUserEmailSQL(ctx, dbpool, w, r, user.Ind_num_taxp, user.Name_of_company, user.Address_name, storedKesh.Email, user.Password_hash, user.Data, file_path)
		}

		return
	}
}

func SignupLegalPhoneCreater(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		op := "internal.services.user.EnterCodeFromEmailCreater"

		// Попытка прочитать куку
		token, err := r.Cookie("token")
		if err != nil {
			if err == http.ErrNoCookie {
				logger.Err(err).Msg(" error in " + op + "; Кука не найдена")

				return
			}

			logger.Err(err).Msg(" error in " + op + "; Проблема с чтением куки")

			return
		}

		if token.Value == "" {
			logger.Err(err).Msg(" error in " + op + "; Токен пуст")

			return
		}

		token_flag, _, _ := jwt.IsAuthorized(token.Value)
		if !token_flag {
			logger.Err(err).Msg(" error in " + op + "; Неверный токен")

			return
		}

		handler := &SignupHandler{
			RedisClient: redisClient,
			Logger:      logger,
			JWT:         token.Value,
		}

		// 4) Вызываем дальше нужную логику: например, метод EnterCodeFromEmail
		subHandler := handler.SignupLegalPhone(ctx, dbpool)
		subHandler(w, r, ps) // передаём управление вашему вторичному хендлеру
	}
}

func (h *SignupHandler) SignupLegalPhone(ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.user.SignupLegalPhone"

		var user model.LegalUser

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&user)
		if err != nil {
			h.Logger.Err(err).Msg(" error in " + op + "; Ошибка при парсинге JSON-запроса")

			return
		}

		flag := function.ValidatePassword(w, user.Password_hash)

		if !flag {
			return
		}

		// Получаем данные из Redis
		keshData, err := h.RedisClient.Get(ctx, h.JWT).Result()
		if err == redis.Nil {
			h.Logger.Err(err).Msg(" error in " + op + "; Код не найден или истек")

			return
		} else if err != nil {
			h.Logger.Err(err).Msg(" error in " + op + "; Ошибка при получении данных из Redis")

			return
		}

		// Десериализуем JSON обратно в структуру kesh
		var storedKesh model.Phone_kesh
		err = json.Unmarshal([]byte(keshData), &storedKesh)
		if err != nil {
			h.Logger.Err(err).Msg(" error in " + op + "; Ошибка при десериализации данных из Redis")

			return
		}

		type Response struct {
			Status  string `json:"status"`
			Data    int    `json:"data"`
			Message string `json:"message"`
		}

		if storedKesh.Phone == "" || user.Ind_num_taxp == 0 || user.Name_of_company == "" || user.Address_name == "" {
			response := Response{
				Status:  "fatal",
				Data:    0,
				Message: "Передаёшь пустое значение",
			}

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)

			return
		}

		repo := database.NewRepo(ctx, dbpool)

		var pwd = "/root/home/beeline_project/media/user/"

		err, image_flag, file_path := function.UploadAvatar(w, user.Avatar, pwd, strings.Split(storedKesh.Phone, ".")[0], "ava")
		if err != nil {
			h.Logger.Err(err).Msg(" error in " + op + "; Ошибка при добавлении аватара. Путь к файлу: " + file_path)
		}
		if image_flag {
			err = repo.SigLegalUserPhoneSQL(ctx, dbpool, w, r, user.Ind_num_taxp, user.Name_of_company, user.Address_name, storedKesh.Phone, user.Password_hash, user.Data, file_path)
		}
		return
	}
}

func SignupNaturEmailCreater(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		op := "internal.services.user.EnterCodeFromEmailCreater"

		// Попытка прочитать куку
		token, err := r.Cookie("token")
		if err != nil {
			if err == http.ErrNoCookie {
				logger.Err(err).Msg(" error in " + op + "; Кука не найдена")

				return
			}

			logger.Err(err).Msg(" error in " + op + "; Проблема с чтением куки")

			return
		}

		if token.Value == "" {
			logger.Err(err).Msg(" error in " + op + "; Токен пуст")

			return
		}

		token_flag, _, _ := jwt.IsAuthorized(token.Value)
		if !token_flag {
			logger.Err(err).Msg(" error in " + op + "; Неверный токен")

			return
		}

		handler := &SignupHandler{
			RedisClient: redisClient,
			Logger:      logger,
			JWT:         token.Value,
		}

		// 4) Вызываем дальше нужную логику: например, метод EnterCodeFromEmail
		subHandler := handler.SignupNaturEmail(ctx, dbpool)
		subHandler(w, r, ps) // передаём управление вашему вторичному хендлеру
	}
}

func (h *SignupHandler) SignupNaturEmail(ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.user.SignupNaturEmail"

		var user model.NaturUser

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&user)
		if err != nil {
			h.Logger.Err(err).Msg(fmt.Sprintf("error in %v; Ошибка при парсинге JSON-запроса", op))

			return
		}

		flag := function.ValidatePassword(w, user.Password_hash)

		if !flag {
			return
		}

		token_flag, _, _ := jwt.IsAuthorized(h.JWT)

		if !token_flag {
			h.Logger.Err(err).Msg(" error in " + op + "; JWT не валиден")

			return
		}

		// Получаем данные из Redis
		keshData, err := h.RedisClient.Get(ctx, h.JWT).Result()
		if err == redis.Nil {
			h.Logger.Err(err).Msg(" error in " + op + "; Код Redis не найден или истек")

			return
		} else if err != nil {
			h.Logger.Err(err).Msg(" error in " + op + "; Ошибка при получении данных из Redis")

			return
		}

		// Десериализуем JSON обратно в структуру kesh
		var storedKesh model.Email_kesh
		err = json.Unmarshal([]byte(keshData), &storedKesh)
		if err != nil {
			h.Logger.Err(err).Msg(" error in " + op + "; Ошибка при десериализации данных из Redis")

			return
		}

		repo := database.NewRepo(ctx, dbpool)

		var pwd = "/root/home/beeline_project/media/user/"

		err, image_flag, file_path := function.UploadAvatar(w, user.Avatar, pwd, strings.Split(storedKesh.Email, ".")[0], "ava")
		if err != nil {
			h.Logger.Err(err).Msg(" error in " + op + "; Ошибка при добавлении аватара. Путь до файла: " + file_path)

			return
		}
		if image_flag {
			err = repo.SigNaturUserEmailSQL(
				ctx,
				dbpool,
				w,
				r,
				user.Name,
				user.Surname,
				user.Patronymic,
				storedKesh.Email,
				user.Password_hash,

				user.Data,
				file_path)

			if err != nil {
				h.Logger.Err(err).Msg(" error in " + op + "; Ошибка с аватаркой пользователя")

				return
			}
		}

		return
	}
}

func SignupNaturPhone(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.user.SignupNaturEmail"

		var user model.NaturUser

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&user)
		if err != nil {
			logger.Err(err).Msg(" error in " + op + "; Ошибка с парсингом JSON-запроса")

			return
		}

		flag := function.ValidatePassword(w, user.Password_hash)

		if !flag {
			return
		}

		// Попытка прочитать куку
		token, err := r.Cookie("token")
		if err != nil {
			logger.Err(err).Msg(" error in " + op + "; Ошибка с попыткой прочитать куку")

			return
		}

		token_flag, _, _ := jwt.IsAuthorized(token.Value)

		if !token_flag {
			fmt.Println("Что-то не так с токеном")
		}

		// Получаем данные из Redis
		keshData, err := redisClient.Get(ctx, token.Value).Result()
		if err == redis.Nil {
			logger.Err(err).Msg(" error in " + op + "; Код не найден или истек")

			return
		} else if err != nil {
			logger.Err(err).Msg(" error in " + op + "; Ошибка при получении данных из Redis")

			return
		}

		// Десериализуем JSON обратно в структуру kesh
		var storedKesh model.Phone_kesh
		err = json.Unmarshal([]byte(keshData), &storedKesh)
		if err != nil {
			logger.Err(err).Msg(" error in " + op + "; Ошибка при десериализации данных из Redis")

			return
		}

		repo := database.NewRepo(ctx, dbpool)

		var pwd = "/root/home/beeline_project/media/user/"

		err, image_flag, file_path := function.UploadAvatar(w, user.Avatar, pwd, strings.Split(storedKesh.Phone, ".")[0], "ava")
		if err != nil {
			logger.Err(err).Msg(" error in " + op + "; Ошибка при добавлениие аватарки. Путь: " + file_path)

			return
		}

		if image_flag {
			err = repo.SigNaturUserPhoneSQL(
				ctx,
				dbpool,
				w,
				r,
				user.Name,
				user.Surname,
				user.Patronymic,
				storedKesh.Phone,
				user.Password_hash,

				user.Data,
				file_path,
			)
		}

		return
	}
}

func EditingLegalUserData(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.user.SignupNaturEmail"

		var legalUser model.LegalUser

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&legalUser)
		if err != nil {
			logger.Err(err).Msg(" error in " + op + "; Ошибка при парсинге JSON-запроса")

			return
		}

		repo := database.NewRepo(ctx, dbpool)

		token, err := jwt.ReadCookie("token", r)
		if err != nil {
			logger.Err(err).Msg(" error in " + op + "; Ошибка при чтении куки")

			return
		}

		flag, user_id, user_role := jwt.IsAuthorized(token)

		if user_role != 1 {
			response := model.Response{
				Status:  "fatal",
				Message: "У вас нет доступа на эту операцию",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		if !flag {
			response := model.Response{
				Status:  "fatal",
				Message: "Что-то не так с JWT",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		if legalUser.Avatar != "" {
			file_path := "/root/home/beeline_project/media/user/"
			err, ava_flag, pwd := function.UploadAvatar(w, legalUser.Avatar, file_path, strconv.Itoa(user_id), "ava")

			if ava_flag {
				err = repo.EditingLegalUserAvaSQL(ctx, w, dbpool, user_id, pwd)

				if err != nil {
					logger.Err(err).Msg(" error in " + op + "; Ошибка при изменении аватара юридического лица. Путь: " + file_path)

					return
				}
			}
		}

		if legalUser.Ind_num_taxp != 0 {
			err = repo.EditingLegalUserIndNumSQL(ctx, w, dbpool, user_id, legalUser.Ind_num_taxp)

			if err != nil {
				logger.Err(err).Msg(" error in " + op + "; Ошибка при изменении ИНН юридического лица")

				return
			}
		}
		if legalUser.Name_of_company != "" {
			err = repo.EditingLegalUserNameCompSQL(ctx, w, dbpool, user_id, legalUser.Name_of_company)

			if err != nil {
				logger.Err(err).Msg(" error in " + op + "; Ошибка при изменении имени компании юридического лица")

				return
			}
		}
		if legalUser.Address_name != "" {
			err = repo.EditingLegalUserAddressNameSQL(ctx, w, dbpool, user_id, legalUser.Address_name)

			if err != nil {
				logger.Err(err).Msg(" error in " + op + "; Ошибка при изменении адреса юридического лица")

				return
			}
		}

	}
}

func EditingNaturUserData(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.user.EditingNaturUserData"

		var naturUser model.NaturUser

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&naturUser)
		if err != nil {
			logger.Err(err).Msg(" error in " + op + "; Ошибка при парсинге JSON-запроса")

			return
		}

		repo := database.NewRepo(ctx, dbpool)

		token, err := jwt.ReadCookie("token", r)
		if err != nil {
			logger.Err(err).Msg(" error in " + op + "; Ошибка при чтении куки")

			return
		}

		flag, user_id, user_role := jwt.IsAuthorized(token)

		if user_role != 1 {
			response := model.Response{
				Status:  "fatal",
				Message: "У вас нет доступа на эту операцию",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		if !flag {
			response := model.Response{
				Status:  "fatal",
				Message: "Что-то не так с JWT",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		if naturUser.Avatar != "" {
			file_path := "/root/home/beeline_project/media/user/"
			err, ava_flag, pwd := function.UploadAvatar(w, naturUser.Avatar, file_path, strconv.Itoa(user_id), "ava")

			if ava_flag {
				err = repo.EditingNaturUserAvaSQL(ctx, w, dbpool, user_id, pwd)

				if err != nil {
					logger.Err(err).Msg(" error in " + op + "; Ошибка при изменении аватара. Физическое лицо")

					return
				}
			}
		}

		if naturUser.Surname != "" {
			err = repo.EditingNaturUserSurnameSQL(ctx, w, dbpool, user_id, naturUser.Surname)

			if err != nil {
				logger.Err(err).Msg(" error in " + op + "; Ошибка при изменении фамилии. Физическое лицо")

				return
			}
		}
		if naturUser.Name != "" {
			err = repo.EditingNaturUserNameSQL(ctx, w, dbpool, user_id, naturUser.Name)

			if err != nil {
				logger.Err(err).Msg(" error in " + op + "; Ошибка при изменении имени. Физическое лицо")

				return
			}
		}
		if naturUser.Patronymic != "" {
			err = repo.EditingNaturUserPatronomSQL(ctx, w, dbpool, user_id, naturUser.Patronymic)

			if err != nil {
				logger.Err(err).Msg(" error in " + op + "; Ошибка при изменении отчества. Физическое лицо")

				return
			}
		}

	}
}

func SendCodForEmail(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.user.EditingNaturUserData"

		var email model.Email_name

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&email)
		if err != nil {
			logger.Err(err).Msg(" error in " + op + "; Ошибка при парсинге JSON-запроса")

			return
		}

		// Настройки SMTP-сервера
		smtpHost := "smtp.mail.ru"
		smtpPort := "587"

		// Данные отправителя (ваша почта и пароль приложения)
		senderEmail := "parpatt_test@mail.ru"
		password := "X0h72ndPXchhjWZ4vbyT" // Пароль приложения

		// Получатель
		recipientEmail := email.Email_name

		// Сообщение
		subject := "Subject: Тебя беспокоит служба безопасности сбербанка.\n"
		body := "Введи этот код.\n"
		codeNum := 777 // Здесь лучше использовать случайный код
		message := []byte(subject + "\n" + body + strconv.Itoa(codeNum))

		// Авторизация для отправки email
		auth := smtp.PlainAuth("", senderEmail, password, smtpHost)

		// Устанавливаем обычное нешифрованное соединение
		client, err := smtp.Dial(smtpHost + ":" + smtpPort)
		if err != nil {
			logger.Err(err).Msg(" error in " + op + "; Ошибка при истановке соединения через SMTP")

			return
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
		mess, err := client.Data()
		if err != nil {
			log.Fatal(err)
		}

		_, err = w.Write(message)
		if err != nil {
			log.Fatal(err)
		}

		err = mess.Close()
		if err != nil {
			log.Fatal(err)
		}

		// Завершение сеанса
		client.Quit()

		// Генерация JWT токена
		ValidToken_jwt, err := jwt.GenerateJWT("jwt_for_proof", 0, 0)
		if err != nil {
			logger.Err(err).Msg(" error in " + op + "; Ошибка генерации токена произошла ошибка")

			return
		}

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
		keshData, err := json.Marshal(Kesh{CodeNum: CodeNum, Email_name: email.Email_name})
		if err != nil {
			logger.Err(err).Msg(" error in " + op + "; Ошибка при преобразовании структуры kesh в JSON")

			return
		}

		// Сохраняем код подтверждения в Redis с TTL 10 минут
		err = redisClient.Set(ctx, ValidToken_jwt, keshData, 10*time.Minute).Err()
		if err != nil {
			logger.Err(err).Msg(" error in " + op + "; Ошибка при сохрании в Redis")

			return
		}

		type Data struct {
			CodeNum        int    `json:"CodeNum"`
			Email_name     string `json:"Email_name"`
			ValidToken_jwt string `json:"ValidToken_jwt"`
		}

		w.WriteHeader(http.StatusOK)
		type Response struct {
			Status  string `json:"status"`
			Data    Data   `json:"data,omitempty"`
			Message string `json:"message"`
		}

		if ValidToken_jwt == "" {
			response := Response{
				Status:  "fatal",
				Message: "Почта не принята",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		response := Response{
			Status:  "success",
			Data:    Data{CodeNum, email.Email_name, ValidToken_jwt},
			Message: "Почта принята",
		}

		json.NewEncoder(w).Encode(response)

		return
	}
}

func EnterCodFromEmail(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.user.EnterCodFromEmail"

		var email model.Reg_code

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&email)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		repo := database.NewRepo(ctx, dbpool)

		// Попытка прочитать куку
		token, err := r.Cookie("token")
		if err != nil {
			logger.Err(err).Msg(" error in " + op + "; Ошибка при попытке прочитать куки")

			return
		}

		token_flag, user_id, user_role := jwt.IsAuthorized(token.Value)

		if !token_flag || user_role != 1 {
			fmt.Println("Что-то не так с токеном")
		}

		// Попытка прочитать куку
		token_kesh, err := r.Cookie("token")
		if err != nil {
			logger.Err(err).Msg(" error in " + op + "; Ошибка при попытке прочитать куки")

			return
		}

		token_flag, _, _ = jwt.IsAuthorized(token_kesh.Value)

		if !token_flag {
			logger.Err(err).Msg(" error in " + op + "; Ошибка при попытке прочитать JWT")

			return
		}

		// Получаем данные из Redis
		kash, err := redisClient.Get(ctx, token_kesh.Value).Result()
		if err == redis.Nil {
			log.Println("Ключ не найден")
			http.Error(w, "Код не найден или истек", http.StatusUnauthorized)
			return
		} else if err != nil {
			log.Fatal("Ошибка при получении данных из Redis:", err)
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}

		// Структура для хранения данных
		var data map[string]interface{}

		// Парсинг JSON в структуру
		err = json.Unmarshal([]byte(kash), &data)
		if err != nil {
			fmt.Println("Ошибка парсинга JSON:", err)
			return
		}

		// Извлечение значения "Phone_num"
		if phoneNum, ok := data["Email_name"].(string); ok {
			if value, ok := data["CodeNum"].(float64); ok {
				if email.Reg_code == int(value) {
					err := repo.EnterCodFromEmailSQL(ctx, w, dbpool, user_id, phoneNum)
					if err != nil {
						logger.Err(err).Msg(" error in " + op + "; Ошибка при попытке извлечение значения Phone_num из JWT")

						return
					}
				}
			} else {
				fmt.Println("CodeNum не является int")
			}
		} else {
			fmt.Println("Email_name не найден или имеет неверный тип")
		}
	}
}

func (h *SignupHandler) SendCodForPhoneNum(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.user.SendCodForPhoneNum"

		var phone model.Phone_num

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&phone)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Генерация случайного кода
		// Диапазон четырёхзначных чисел: от 1000 до 9999
		min, max := 1000, 9999
		// Вычисляем размер диапазона
		rangeSize := big.NewInt(int64(max - min + 1))
		// Генерируем случайное число в диапазоне от 0 до rangeSize-1
		n, err := rand.Int(rand.Reader, rangeSize)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s", op+"; Ошибка генерации случайного числа"))

			return
		}
		// Смещаем результат, чтобы получить число в диапазоне от min до max
		h.CodeNum = int(n.Int64() + int64(min)) // искомое число

		request_url := "https://zvonok.com/manager/cabapi_external/api/v1/phones/flashcall/"

		// Создание буфера для тела запроса
		var requestBody bytes.Buffer
		writer := multipart.NewWriter(&requestBody)

		// Добавление полей в multipart-запрос
		writer.WriteField("public_key", "ba885d6d0342490a50c6bf5603d75719")
		writer.WriteField("phone", phone.Phone_num)
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
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; Ошибка при попытке генерации JWT токена", op))

			return
		}

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

		_ = cookie

		type Kesh struct {
			CodeNum   int
			Phone_num string
		}

		// Преобразуем структуру kesh в JSON
		keshData, err := json.Marshal(Kesh{CodeNum: h.CodeNum, Phone_num: phone.Phone_num})
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; Ошибка при преобразуем структуру kesh в JSON", op))

			return
		}

		// Сохраняем код подтверждения в Redis с TTL 10 минут
		err = redisClient.Set(ctx, ValidToken_jwt, keshData, 10*time.Minute).Err()
		if err != nil {
			logger.Err(err).Msg(" error in " + op + "; Ошибка при работе с Redis")

			return
		}

		w.WriteHeader(http.StatusOK)

		if ValidToken_jwt == "" || phone.Phone_num == "" {
			response := model.Response{
				Status:  "fatal",
				Message: "Телефон не принят",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		response := model.Response_SendCodForPhoneNum{
			Status:  "success",
			Data:    model.Data{CodeNum: h.CodeNum, Phone_num: phone.Phone_num, ValidToken_jwt: ValidToken_jwt},
			Message: "Почта принята",
		}

		json.NewEncoder(w).Encode(response)

		return
	}
}

func EnterCodFromPhoneNum(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.user.EnterCodFromPhoneNum"

		var phone model.Reg_code

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&phone)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		repo := database.NewRepo(ctx, dbpool)

		// Попытка прочитать куку
		token, err := r.Cookie("token")
		if err != nil {
			logger.Err(err).Msg(" error in " + op + "; Ошибка при попытка прочитать куку")

			return
		}

		token_flag, user_id, user_role := jwt.IsAuthorized(token.Value)

		if !token_flag || user_role != 1 {
			logger.Err(err).Msg(" error in " + op + "; Ошибка при попытка прочитать JWT")

			return

		}

		// Попытка прочитать куку
		token_kesh, err := r.Cookie("token_kesh")
		if err != nil {
			logger.Err(err).Msg(" error in " + op + "; Ошибка при попытка прочитать куку")

			return
		}

		token_flag, _, _ = jwt.IsAuthorized(token_kesh.Value)

		if !token_flag {
			logger.Err(err).Msg(" error in " + op + "; Ошибка при попытка прочитать JWT")

			return
		}

		// Получаем данные из Redis
		kash, err := redisClient.Get(ctx, token_kesh.Value).Result()
		if err == redis.Nil {
			log.Println("Ключ не найден")
			http.Error(w, "Код не найден или истек", http.StatusUnauthorized)
			return
		} else if err != nil {
			log.Fatal("Ошибка при получении данных из Redis:", err)
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}

		// Структура для хранения данных
		var data map[string]interface{}

		// Парсинг JSON в структуру
		err = json.Unmarshal([]byte(kash), &data)
		if err != nil {
			fmt.Println("Ошибка парсинга JSON:", err)
			return
		}

		// Извлечение значения "Phone_num"
		if phoneNum, ok := data["Phone_num"].(string); ok {
			if value, ok := data["CodeNum"].(float64); ok {
				if phone.Reg_code == int(value) {
					err := repo.EnterCodFromPhoneNumSQL(ctx, w, dbpool, user_id, phoneNum)
					if err != nil {
						logger.Err(err).Msg(" error in " + op + "; Ошибка при попытке извлечение значения Phone_num из JWT")

						return
					}
				}
			} else {
				logger.Err(err).Msg(" error in " + op + "; CodeNum не является int")

				return
			}
		} else {
			logger.Err(err).Msg(" error in " + op + "; Phone_num не найден или имеет неверный тип")

			return
		}
	}
}

func FavProfilsFirstNew(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.user.FavProfilsFirstNew"

		repo := database.NewRepo(ctx, dbpool)

		// Попытка прочитать куку
		token, err := r.Cookie("token")
		if err != nil {
			logger.Err(err).Msg(" error in " + op + "; Ошибка при попытка прочитать куку")

			return
		}

		token_flag, user_id, user_role := jwt.IsAuthorized(token.Value)

		if user_role != 1 {
			response := model.Response{
				Status:  "fatal",
				Message: "У вас нет доступа на эту операцию",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		if !token_flag {
			response := model.Response{
				Status:  "fatal",
				Message: "Что-то не так с JWT",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		err = repo.FavProfilsFirstNewSQL(ctx, w, dbpool, r, user_id)

		if err != nil {

		}
	}
}

func FavProfilsFirstOld(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.user.FavProfilsFirstOld"

		repo := database.NewRepo(ctx, dbpool)

		// Попытка прочитать куку
		token, err := r.Cookie("token")
		if err != nil {
			logger.Err(err).Msg(" error in " + op + "; Ошибка при попытка прочитать куку")

			return
		}

		token_flag, user_id, user_role := jwt.IsAuthorized(token.Value)

		if user_role != 1 {
			response := model.Response{
				Status:  "fatal",
				Message: "У вас нет доступа на эту операцию",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		if !token_flag {
			response := model.Response{
				Status:  "fatal",
				Message: "Что-то не так с JWT",
			}

			json.NewEncoder(w).Encode(response)

			return
		}
		err = repo.FavProfilsFirstOldSQL(ctx, w, dbpool, r, user_id)

		if err != nil {

		}
	}
}

func FavProfilsFirstCheap(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.user.FavProfilsFirstCheap"

		repo := database.NewRepo(ctx, dbpool)

		// Попытка прочитать куку
		token, err := r.Cookie("token")
		if err != nil {
			logger.Err(err).Msg(" error in " + op + "; Ошибка при попытка прочитать куку")

			return
		}

		token_flag, user_id, user_role := jwt.IsAuthorized(token.Value)

		if user_role != 1 {
			response := model.Response{
				Status:  "fatal",
				Message: "У вас нет доступа на эту операцию",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		if !token_flag {
			response := model.Response{
				Status:  "fatal",
				Message: "Что-то не так с JWT",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		err = repo.FavProfilsFirstCheapSQL(ctx, w, dbpool, r, user_id)

		if err != nil {

		}
	}
}

func FavProfilsFirstDearl(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.user.FavProfilsFirstDearl"

		repo := database.NewRepo(ctx, dbpool)

		// Попытка прочитать куку
		token, err := r.Cookie("token")
		if err != nil {
			logger.Err(err).Msg(" error in " + op + "; Ошибка при попытка прочитать куку")

			return
		}

		token_flag, user_id, user_role := jwt.IsAuthorized(token.Value)

		if user_role != 1 {
			response := model.Response{
				Status:  "fatal",
				Message: "У вас нет доступа на эту операцию",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		if !token_flag {
			response := model.Response{
				Status:  "fatal",
				Message: "Что-то не так с JWT",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		err = repo.FavProfilsFirstDearlSQL(ctx, w, dbpool, r, user_id)

		if err != nil {

		}
	}
}

func OpenUserProfile(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.user.FavProfilsFirstDearl"

		var user model.User_id

		repo := database.NewRepo(ctx, dbpool)

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&user)
		if err != nil {
			logger.Err(err).Msg(" error in " + op + "; Ошибка при парсинге JSON-запроса")

			return
		}

		err = repo.OpenUserProfileSQL(ctx, w, dbpool, r, user.User_id)

		if err != nil {
			logger.Err(err).Msg("Error in " + op + "; Ошибка в хендлере")

			return
		}
	}
}
