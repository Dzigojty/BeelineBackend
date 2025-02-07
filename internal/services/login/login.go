package login

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	function "myproject/internal"
	"myproject/internal/database"
	"myproject/internal/jwt"
	"myproject/internal/model"
	"net/http"
	"net/smtp"
	"net/url"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/yandex"
)

type SignupHandler struct {
	RedisClient *redis.Client
	Logger      zerolog.Logger
	CodeNum     int
}

// Создайте конфигурацию OAuth2
var oauthConf = &oauth2.Config{
	ClientID:     "5dfe4f3e5c474d1ea9c734a79407d250",
	ClientSecret: "0695e988c1084c19af3b16b6a1ffcdf0",
	RedirectURL:  "http://localhost:8070/callback",
	Scopes:       []string{"login:email", "login:info"},
	Endpoint:     yandex.Endpoint,
}

func LoginYandex(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.login.LoginYandex"

		// Лог с контекстом
		logger.Info().
			Str("service", "loginYandex").
			Int("port", 8090).
			Msg(fmt.Sprintf("User login, in %s", op))

		stateToken := "FromOCTA" + time.Now().String() + "FromDzigo"

		url := oauthConf.AuthCodeURL(stateToken, oauth2.AccessTypeOffline)
		http.Redirect(w, r, url, http.StatusTemporaryRedirect)
	}
}

func Callback(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.login.Callback"

		// Лог с контекстом
		logger.Info().
			Str("service", "callback").
			Int("port", 8090).
			Msg(fmt.Sprintf("Callback from Yandex, in %s", op))

		code := r.URL.Query().Get("code") // Извлекает код авторизации из параметров запроса
		if code == "" {
			logger.Error().Msg(fmt.Sprintf("Не удалось получить код авторизации, %d", http.StatusBadRequest))

			return
		}

		// Обмен кода на токен
		token, err := oauthConf.Exchange(context.Background(), code)
		if err != nil {
			http.Error(w, "Не удалось обменять код на токен: %d"+err.Error(), http.StatusInternalServerError)
			return
		}

		req, _ := http.NewRequest("GET", "https://login.yandex.ru/info", nil)
		req.Header.Set("Authorization", "Bearer "+token.AccessToken)
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			log.Fatal(err)
		}
		defer resp.Body.Close()
		// Обработайте ответ (имя, фамилия и т.д.)

		var userInfo model.YandexUserInfo
		if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
			log.Fatalf("Ошибка декодирования ответа: %v", err)
		}

		// Лог с контекстом
		logger.Info().
			Str("service", "login").
			Int("port", 8090).
			Msg("User login")

		repo := database.NewRepo(ctx, dbpool)

		err = repo.CallbackSQL(ctx, dbpool, w, logger, userInfo)
		if err != nil {
			logger.Err(err).Msg(" error in " + op + "; Ошибка при попытке залогиниться")

			return
		}
	}
}

func Login(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.login.Login"

		var login model.Login

		repo := database.NewRepo(ctx, dbpool)

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&login)
		if err != nil {
			logger.Err(err).Msg(" error in " + op + "; Ошибка при парсинге JSON-запроса")

			return
		}

		// Лог с контекстом
		logger.Info().
			Str("service", "login").
			Int("port", 8090).
			Msg("User login")

		err = repo.LoginSQL(ctx, dbpool, w, login.Login, login.Password, logger)
		if err != nil {
			logger.Err(err).Msg(" error in " + op + "; Ошибка при попытке залогиниться")

			return
		}
	}
}

func SendCodeForRecoveryPassWithEmail(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.login.SendCodeForRecoveryPassWithEmail"

		var passwd model.Passwd

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&passwd)
		if err != nil {
			logger.Err(err).Msg(" error in " + op + "; Ошибка при парсинге JSON-запроса")

			return
		}

		repo := database.NewRepo(ctx, dbpool)

		// Попытка прочитать куку
		token, err := r.Cookie("token")
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %v; ошибка с попыткой прочитать куку", op))

			return
		}

		token_flag, user_id, _ := jwt.IsAuthorized(token.Value)

		if !token_flag {
			response := model.Response{
				Status:  "fatal",
				Message: "Что-то не так с JWT",
			}

			json.NewEncoder(w).Encode(response)

			return
		}
		if passwd.Passwd_1 == passwd.Passwd_2 {
			flag := function.ValidatePassword(w, passwd.Passwd_1)

			if flag {
				logger.Err(err).Msg(fmt.Sprintf("error in ", op, "; пароль не валиден"))

				return
			}

			// Генерация JWT токена
			Jwt_for_proof, err := jwt.GenerateJWT("jwt_for_reg", 0, 0)
			if err != nil {
				logger.Err(err).Msg(fmt.Sprintf("error in ", op, "; ошибка с генерацией JWT токена"))

				return
			}

			// Установка куки
			livingTime := 20 * time.Minute //не смог найти чего-то получше
			expiration := time.Now().Add(livingTime)
			cookie := http.Cookie{
				Name:     "token",
				Value:    url.QueryEscape(Jwt_for_proof),
				Expires:  expiration,
				Path:     "/",             // Убедитесь, что путь корректен
				Domain:   "185.112.83.36", // IP-адрес вашего сервера
				HttpOnly: true,
				Secure:   false, // Для HTTP можно оставить false
				SameSite: http.SameSiteLaxMode,
			}

			fmt.Printf("Кука установлена: %v\n", cookie)

			err = repo.RecoveryPassWithEmailSQL(ctx, w, dbpool, redisClient, user_id, passwd.Passwd_1, Jwt_for_proof)
			if err != nil {
				logger.Err(err).Msg(fmt.Sprintf("error in %s", op, "; ошибка с процедурой RecoveryPassWithEmailSQL"))
			}

			return
		}
		type Response struct {
			Status  string `json:"status"`
			Message string `json:"message"`
		}

		response := Response{
			Status:  "fatal",
			Message: "Поля не совпадают!",
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}
}

func EnterCodeForRecoveryPassWithEmail(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.login.EnterCodeForRecoveryPassWithEmail"

		var email model.Reg_code

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&email)
		if err != nil {
			logger.Err(err).Msg(" error in " + op + "; Ошибка при парсинге JSON-запроса")

			return
		}

		repo := database.NewRepo(ctx, dbpool)

		token, err := r.Cookie("token")

		token_flag, user_id, _ := jwt.IsAuthorized(token.Value)

		Jwt_for_proof, err := r.Cookie("jwt_for_proof")

		Jwt_for_proof_flag, _, _ := jwt.IsAuthorized(Jwt_for_proof.Value)

		if !token_flag {
			response := model.Response{
				Status:  "fatal",
				Message: "У вас нет доступа на эту операцию",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		if !Jwt_for_proof_flag {
			response := model.Response{
				Status:  "fatal",
				Message: "Что-то не так с JWT",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		// Получаем данные из Redis
		keshData, err := redisClient.Get(ctx, Jwt_for_proof.Value).Result()
		if err == redis.Nil {
			logger.Err(err).Msg(fmt.Sprintf(" error in "+op+"; Ошибка: %v. Код не найден или истек: %d", err, http.StatusUnauthorized))

			return
		} else if err != nil {
			logger.Err(err).Msg(fmt.Sprintf(" error in "+op+"; Ошибка: %v. Ошибка при получении данных из Redis: %d", err, http.StatusInternalServerError))

			return
		}

		// Десериализуем JSON обратно в структуру kesh
		var storedKesh model.Kesh_passwd_code
		err = json.Unmarshal([]byte(keshData), &storedKesh)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf(" error in "+op+"; Ошибка: %v. Ошибка при десериализации данных из Redis: %d", err, http.StatusInternalServerError))

			return
		}

		// Проверяем код
		if email.Reg_code == storedKesh.Code {
			err = repo.EnterCodeForRecoveryPassWithEmailSQL(ctx, w, dbpool, user_id, storedKesh.Passwd)
			if err != nil {
				logger.Err(err).Msg(fmt.Sprintf(" error in %s; Ошибка при проверке кода", op))

				return
			}
		} else {
			logger.Info().Msg(fmt.Sprintf("error in %s; был введен неверный код подтверждения", op))
		}
	}
}

func SendCodeForRecoveryPassWithPhoneNum(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.login.SendCodeForRecoveryPassWithPhoneNum"

		var passwd model.Passwd

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&passwd)
		if err != nil {
			logger.Err(err).Msg(" error in %s; Ошибка при парсинге JSON-запроса")

			return
		}

		repo := database.NewRepo(ctx, dbpool)

		// Попытка прочитать куку
		token, err := r.Cookie("token")
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in ", op, "; ошибка с попыткой прочитать куку"))

			return
		}

		token_flag, user_id, _ := jwt.IsAuthorized(token.Value)

		if !token_flag {
			response := model.Response{
				Status:  "fatal",
				Message: "Что-то не так с JWT",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		if passwd.Passwd_1 == passwd.Passwd_2 {
			flag := function.ValidatePassword(w, passwd.Passwd_1)

			if flag {
				return
			}
			// Генерация JWT токена
			Jwt_for_proof, err := jwt.GenerateJWT("jwt_for_reg", 0, 0)
			if err != nil {
				logger.Err(err).Msg(fmt.Sprintf(" error in "+op+"; Ошибка: %v. ошибка с генерацией JWT токена: %d", err))

				return
			}

			// Установка куки
			livingTime := 20 * time.Minute //не смог найти чего-то получше
			expiration := time.Now().Add(livingTime)
			cookie := http.Cookie{
				Name:     "token",
				Value:    url.QueryEscape(Jwt_for_proof),
				Expires:  expiration,
				Path:     "/",             // Убедитесь, что путь корректен
				Domain:   "185.112.83.36", // IP-адрес вашего сервера
				HttpOnly: true,
				Secure:   false, // Для HTTP можно оставить false
				SameSite: http.SameSiteLaxMode,
			}

			_ = cookie

			err = repo.RecoveryPassWithPhoneNumSQL(ctx, w, dbpool, redisClient, user_id, passwd.Passwd_1, Jwt_for_proof)
			if err != nil {
				logger.Err(err).Msg(fmt.Sprintf("error in %s", op, "; ошибка с процедурой RecoveryPassWithPhoneNumSQL"))
			}

			return
		}
		response := model.Response{
			Status:  "fatal",
			Message: "Поля не совпадают!",
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}
}

func EnterCodeForRecoveryPassWithPhoneNum(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.login.EnterCodeForRecoveryPassWithPhoneNum"

		var email model.Reg_code

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&email)
		if err != nil {
			logger.Err(err).Msg(" error in " + op + "; Ошибка при парсинге JSON-запроса")

			return
		}

		repo := database.NewRepo(ctx, dbpool)

		token, err := r.Cookie("token")

		token_flag, user_id, _ := jwt.IsAuthorized(token.Value)

		Jwt_for_proof, err := r.Cookie("jwt_for_proof")

		Jwt_for_proof_flag, _, _ := jwt.IsAuthorized(Jwt_for_proof.Value)

		if !token_flag {
			response := model.Response{
				Status:  "fatal",
				Message: "Что-то не так с JWT",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		if !Jwt_for_proof_flag {
			response := model.Response{
				Status:  "fatal",
				Message: "Что-то не так с JWT",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		// Получаем данные из Redis
		keshData, err := redisClient.Get(ctx, Jwt_for_proof.Value).Result()
		if err == redis.Nil {
			log.Println("Ключ не найден")
			http.Error(w, "Код не найден или истек", http.StatusUnauthorized)
			return
		} else if err != nil {
			log.Fatal("Ошибка при получении данных из Redis:", err)
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}

		// Десериализуем JSON обратно в структуру kesh
		var storedKesh model.Kesh_passwd_code
		err = json.Unmarshal([]byte(keshData), &storedKesh)
		if err != nil {
			log.Fatal("Ошибка при десериализации данных из Redis:", err)
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}

		// Проверяем код
		if email.Reg_code == storedKesh.Code {
			err = repo.EnterCodeForRecoveryPassWithEmailSQL(ctx, w, dbpool, user_id, storedKesh.Passwd)
			if err != nil {
				log.Fatal(err)
			}
			w.Write([]byte("Смена завершена успешно"))
		} else {
			http.Error(w, "Неверный код подтверждения", http.StatusUnauthorized)
		}
	}
}

func RecoveryPass(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.login.RecoveryPassWithEmail"

		var passwd model.Passwd

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&passwd)
		if err != nil {
			logger.Err(err).Msg(" error in " + op + "; Ошибка при парсинге JSON-запроса")

			return
		}

		repo := database.NewRepo(ctx, dbpool)

		token, err := r.Cookie("token")

		token_flag, user_id, _ := jwt.IsAuthorized(token.Value)

		if !token_flag {
			response := model.Response{
				Status:  "fatal",
				Message: "Что-то не так с JWT",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		// Проверяем код
		if passwd.Passwd_1 == passwd.Passwd_2 {
			flag := function.ValidatePassword(w, passwd.Passwd_1)

			if !flag {
				logger.Err(err).Msg(" error in " + op + "; пароль не валиден")

				return
			}

			err = repo.RecoveryPassSQL(ctx, w, dbpool, redisClient, user_id, passwd.Passwd_1)
			if err != nil {
				logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой RecoveryPassWithPhoneNumSQL", op))
			}

			w.Write([]byte("Смена завершена успешно"))
		} else {
			http.Error(w, "Неверный код подтверждения", http.StatusUnauthorized)
		}
	}
}

func AutorizLoginEmailSend(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.login.AutorizLoginEmailSend"

		var login model.Login

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&login)
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
		recipientEmail := login.Login

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
		messg, err := client.Data()
		if err != nil {
			log.Fatal(err)
		}

		_, err = w.Write(message)
		if err != nil {
			log.Fatal(err)
		}

		err = messg.Close()
		if err != nil {
			log.Fatal(err)
		}

		// Завершение сеанса
		client.Quit()

		type Kesh struct {
			Login model.Login
			Code  int
		}

		// Преобразуем структуру kesh в JSON
		keshData, err := json.Marshal(Kesh{Login: login, Code: codeNum})
		if err != nil {
			log.Fatal("Ошибка при сериализации структуры kesh:", err)
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}

		// Сохраняем код подтверждения в Redis с TTL 10 минут
		err = redisClient.Set(ctx, "jwt", keshData, 10*time.Minute).Err()
		if err != nil {
			log.Fatal("Ошибка при сохранении кода в Redis:", err)
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Код отправлен на вашу почту."))
	}
}

func AutorizLoginEmailEnter(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.login.AutorizLoginEmailEnter"

		var code model.Reg_code

		repo := database.NewRepo(ctx, dbpool)

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&code)
		if err != nil {
			logger.Err(err).Msg(" error in " + op + "; Ошибка при парсинге JSON-запроса")

			return
		}

		// Получаем данные из Redis
		keshData, err := redisClient.Get(ctx, "jwt").Result()
		if err == redis.Nil {
			log.Println("Ключ не найден")
			http.Error(w, "Код не найден или истек", http.StatusUnauthorized)
			return
		} else if err != nil {
			log.Fatal("Ошибка при получении данных из Redis:", err)
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}

		type Kesh struct {
			Login model.Login
			Code  int
		}

		// Десериализуем JSON обратно в структуру kesh
		var storedKesh Kesh
		err = json.Unmarshal([]byte(keshData), &storedKesh)
		if err != nil {
			log.Fatal("Ошибка при десериализации данных из Redis:", err)
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}

		if storedKesh.Code == code.Reg_code {
			err := repo.LoginSQL(ctx, dbpool, w, storedKesh.Login.Login, storedKesh.Login.Password, logger)
			if err != nil {
				logger.Err(err).Msg(fmt.Sprintf("error in %s", op, "; ошибка с процедурой LoginSQL"))
			}
		}
	}
}

func RefreshToken(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.login.RefreshToken"

		// Попытка прочитать куку
		token, err := r.Cookie("Refresh_token")
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in ", op, "; ошибка с попыткой прочитать куку"))

			return
		}

		flag, user_id, user_role := jwt.IsAuthorized(token.Value)

		if !flag {
			response := model.Response{
				Status:  "fatal",
				Message: "Что-то не так с JWT",
			}

			json.NewEncoder(w).Encode(response)

			return
		}
		// Генерация JWT токена
		validToken_jwt, err := jwt.GenerateJWT("jwt", user_id, user_role)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in ", op, "; ошибка с генерацией JWT токена"))

			return
		}

		// Генерация refresh токена
		refresh_token, err := jwt.GenerateJWT("refresh", user_id, user_role)
		if err != nil {

		}

		type User struct {
			Id            int    `json:"id"`
			JWT           string `json:"JWT"`
			Refresh_token string `json:"Refresh_token"`
		}

		type Response struct {
			Status  string `json:"status"`
			Data    User   `json:"data,omitempty"`
			Message string `json:"message"`
		}

		user := User{
			Id:            user_id,
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

		fmt.Printf("Кука установлена: %v\n", cookie)

		response := Response{
			Status:  "success",
			Data:    user,
			Message: "You have successfully logged in",
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
		return

	}
}

func RecoveryPasswdEmail(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.login.RecoveryPasswdEmail"

		var email model.Email

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&email)
		if err != nil {
			logger.Err(err).Msg(" error in " + op + "; Ошибка при парсинге JSON-запроса")

			return
		}

		repo := database.NewRepo(ctx, dbpool)

		err = repo.RecoveryPasswdEmailSQL(ctx, w, dbpool, r, redisClient, email.Email)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой RecoveryPasswdSQL", op))
		}
	}
}

func RecoveryPasswdPhone(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.login.RecoveryPasswdPhone"

		var email model.Phone_num

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&email)
		if err != nil {
			logger.Err(err).Msg("error in " + op + "; Ошибка при парсинге JSON-запроса")

			return
		}

		handler := &database.SignupHandler{
			RedisClient: redisClient,
			Logger:      logger,
		}

		err = handler.RecoveryPasswdPhoneSQL(ctx, w, dbpool, r, redisClient, email.Phone_num)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой RecoveryPasswdSQL", op))
		}
	}
}

func SendCode(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.login.SendCode"

		var code model.Email_kesh

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&code)
		if err != nil {
			logger.Err(err).Msg(" error in " + op + "; Ошибка при парсинге JSON-запроса")

			return
		}

		repo := database.NewRepo(ctx, dbpool)

		token, err := r.Cookie("ValidToken_jwt")

		token_flag, _, _ := jwt.IsAuthorized(token.Value)

		if !token_flag {
			response := model.Response{
				Status:  "fatal",
				Message: "Что-то не так с JWT",
			}

			json.NewEncoder(w).Encode(response)

			return
		}
		err = repo.SendCodeSQL(ctx, w, dbpool, r, redisClient, token, code.Code)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s", op, "; ошибка с процедурой SendCodeSQL"))
		}
	}
}

func EnterPasswd(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.login.EnterPasswd"

		var passwd model.Passwd

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&passwd)
		if err != nil {
			logger.Err(err).Msg(" error in " + op + "; Ошибка при парсинге JSON-запроса")

			return
		}

		repo := database.NewRepo(ctx, dbpool)

		if passwd.Passwd_1 != passwd.Passwd_2 && passwd.Passwd_2 != "" {
			response := model.Response{
				Status:  "fatal",
				Message: "Пароли не совпадают",
			}

			// w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)

			return
		}

		token, err := r.Cookie("ValidToken_jwt")

		token_flag, _, _ := jwt.IsAuthorized(token.Value)

		if !token_flag {
			response := model.Response{
				Status:  "fatal",
				Message: "Что-то не так с JWT",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		err = repo.EnterPasswdSQL(ctx, w, dbpool, r, redisClient, token, passwd.Passwd_1)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой EnterPasswdSQL", op))
		}
	}
}

func AddAddress(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.login.EnterPasswd"

		var address model.Address

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&address)
		if err != nil {
			logger.Err(err).Msg(" error in " + op + "; Ошибка при парсинге JSON-запроса")

			return
		}

		repo := database.NewRepo(ctx, dbpool)

		token, err := r.Cookie("token")

		token_flag, user_id, _ := jwt.IsAuthorized(token.Value)

		if !token_flag {
			response := model.Response{
				Status:  "fatal",
				Message: "Что-то не так с JWT",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		err = repo.AddAddressSQL(ctx, w, dbpool, r, redisClient, user_id, address.Name)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой EnterPasswdSQL", op))
		}
	}
}

func GiveAddress(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.login.EnterPasswd"

		repo := database.NewRepo(ctx, dbpool)

		token, err := r.Cookie("token")

		token_flag, user_id, _ := jwt.IsAuthorized(token.Value)

		if !token_flag {
			response := model.Response{
				Status:  "fatal",
				Message: "Что-то не так с JWT",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		err = repo.GiveAddressSQL(ctx, w, dbpool, r, redisClient, user_id)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой EnterPasswdSQL", op))
		}
	}
}
