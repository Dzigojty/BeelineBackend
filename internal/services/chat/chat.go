package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"myproject/internal/database"
	"myproject/internal/jwt"
	"myproject/internal/model"
	"net/http"
	"strconv"

	function "myproject/internal"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog"
)

func ChatButtonInAds(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.chat.ChatButtonInAds"

		var sigChat model.Ads_id

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&sigChat)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с парсингом JSON-запроса", op))
		}

		repo := database.NewRepo(ctx, dbpool)

		// Попытка прочитать куку
		token, err := r.Cookie("token")
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с попыткой прочитать куку", op))
		}

		flag, user_id, user_role := jwt.IsAuthorized(token.Value)

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
		err = repo.ChatButtonInAdsSQL(ctx, w, dbpool, user_id, sigChat.Ads_id)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой ChatButtonInAdsSQL", op))
		}
	}
}

func SigChat(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.chat.SigChat"

		var sigChat model.Ads_id

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&sigChat)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с парсингом JSON-запроса", op))
		}

		repo := database.NewRepo(ctx, dbpool)

		// Попытка прочитать куку
		token, err := r.Cookie("token")
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с попыткой прочитать куку", op))
		}

		flag, user_id, user_role := jwt.IsAuthorized(token.Value)

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
		err = repo.SigChatSQL(ctx, w, user_id, sigChat.Ads_id, dbpool)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой SigChatSQL", op))
		}
	}
}

func OpenChat(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.chat.OpenChat"

		var openChat model.Chat

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&openChat)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с парсингом JSON-запроса", op))
		}

		repo := database.NewRepo(ctx, dbpool)

		// Попытка прочитать куку
		token, err := r.Cookie("token")
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с попыткой прочитать куку", op))
		}

		flag, user_id, _ := jwt.IsAuthorized(token.Value)

		if !flag {
			response := model.Response{
				Status:  "fatal",
				Message: "Что-то не так с JWT",
			}

			json.NewEncoder(w).Encode(response)

			return
		}
		err = repo.OpenChatSQL(ctx, w, dbpool, openChat.Id_chat, user_id)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой OpenChatSQL", op))
		}
	}
}

func SendMessageAndImage(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool, сonnections map[int]*websocket.Conn) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.chat.SendMessageAndImage"

		var sendMess model.SendMessAndImg

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&sendMess)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с парсингом JSON-запроса", op))
		}

		repo := database.NewRepo(ctx, dbpool)

		// Попытка прочитать куку
		token, err := r.Cookie("token")
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с попыткой прочитать куку", op))
		}
		flag, user_id, _ := jwt.IsAuthorized(token.Value)

		var pwd = "/home/beeline/media/chat/"

		err, img_flag, file_paths := function.UploadImagesMass(w, sendMess.Images, pwd, strconv.Itoa(user_id), logger) // добавляет изображения
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с при попытке добавить массив медиа", op))
		}

		if !img_flag {
			response := model.Response{
				Status:  "fatal",
				Message: "Ошибка воспроизведения изображения",
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

		err = repo.SendMessageAndMediaSQL(ctx, w, dbpool, r, sendMess.Id_chat, user_id, sendMess.Text, file_paths, сonnections, redisClient)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой SendMessageAndMediaSQL", op))
		}
	}
}

func SendImage(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool, сonnections map[int]*websocket.Conn) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.chat.SendImage"

		var sendMess model.SendImg

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&sendMess)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с парсингом JSON-запроса", op))
		}

		repo := database.NewRepo(ctx, dbpool)

		// Попытка прочитать куку
		token, err := r.Cookie("token")
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с попыткой прочитать куку", op))
		}

		flag, user_id, _ := jwt.IsAuthorized(token.Value)

		var pwd = "/home/beeline/media/chat/"

		err, img_flag, file_paths := function.UploadImagesMass(w, sendMess.Images, pwd, strconv.Itoa(user_id), logger) // добавляет изображения
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с при попытке добавить массив медиа", op))
		}

		if !img_flag {
			response := model.Response{
				Status:  "fatal",
				Message: "Ошибка воспроизведения изображения",
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

		err = repo.SendImageSQL(ctx, w, dbpool, r, sendMess.Id_chat, user_id, file_paths, сonnections, redisClient)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой SendImageSQL", op))
		}
	}
}

func SendMessage(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool, сonnections map[int]*websocket.Conn) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.chat.SendMessage"

		var sendMess model.SendMess

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&sendMess)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с парсингом JSON-запроса", op))
		}

		repo := database.NewRepo(ctx, dbpool)

		// Попытка прочитать куку
		token, err := r.Cookie("token")
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с попыткой прочитать куку", op))
		}

		flag, user_id, _ := jwt.IsAuthorized(token.Value)

		if !flag {
			response := model.Response{
				Status:  "fatal",
				Message: "Что-то не так с JWT",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		err = repo.SendMessageSQL(ctx, w, dbpool, r, сonnections, sendMess.Id_chat, user_id, sendMess.Text, redisClient)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой SendMessageSQL", op))
		}
	}
}

func SendMessageAndVideo(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool, сonnections map[int]*websocket.Conn) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.chat.SendMessageAndVideo"

		var sendMess model.SendMessAndVideo

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&sendMess)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с парсингом JSON-запроса", op))
		}

		repo := database.NewRepo(ctx, dbpool)

		// Попытка прочитать куку
		token, err := r.Cookie("token")
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с попыткой прочитать куку", op))
		}

		flag, user_id, _ := jwt.IsAuthorized(token.Value)

		var pwd = "/home/beeline/media/chat/"

		err, img_flag, file_paths := function.UploadVideosMass(w, sendMess.Videos, pwd, strconv.Itoa(user_id), logger) // добавляет изображения
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с при попытке добавить массив видео", op))
		}

		if !img_flag {
			response := model.Response{
				Status:  "fatal",
				Message: "Ошибка воспроизведения изображения",
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

		err = repo.SendMessageAndMediaSQL(ctx, w, dbpool, r, sendMess.Id_chat, user_id, sendMess.Text, file_paths, сonnections, redisClient)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой SendMessageAndMediaSQL", op))
		}
	}
}

func SendVideo(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool, сonnections map[int]*websocket.Conn) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.chat.SendVideo"

		var sendMess model.SendVideo

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&sendMess)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с парсингом JSON-запроса", op))
		}

		repo := database.NewRepo(ctx, dbpool)

		// Попытка прочитать куку
		token, err := r.Cookie("token")
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с попыткой прочитать куку", op))
		}

		flag, user_id, _ := jwt.IsAuthorized(token.Value)

		var pwd = "/home/beeline/media/chat/"

		err, img_flag, file_paths := function.UploadVideosMass(w, sendMess.Videos, pwd, strconv.Itoa(user_id), logger) // добавляет изображения
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с при попытке добавить массив видео", op))
		}
		if !img_flag {
			response := model.Response{
				Status:  "fatal",
				Message: "Ошибка воспроизведения изображения",
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

		err = repo.SendVideoSQL(ctx, w, dbpool, r, sendMess.Id_chat, user_id, file_paths, сonnections, redisClient)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой SendVideoSQL", op))
		}
	}
}

func SigDisputInChat(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.chat.SigDisputInChat"

		var chat model.Chat

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&chat)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с парсингом JSON-запроса", op))
		}

		repo := database.NewRepo(ctx, dbpool)

		// Попытка прочитать куку
		token, err := r.Cookie("token")
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с попыткой прочитать куку", op))
		}

		flag, user_id, user_role := jwt.IsAuthorized(token.Value)

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

		err = repo.SigDisputInChatSQL(ctx, w, dbpool, chat.Id_chat, user_id)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой SigDisputInChatSQL", op))
		}
	}
}

func PrintChat(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.chat.PrintChat"

		repo := database.NewRepo(ctx, dbpool)

		// Попытка прочитать куку
		token, err := r.Cookie("token")
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с попыткой прочитать куку", op))

			return
		}

		token_flag, user_id, _ := jwt.IsAuthorized(token.Value)

		// if user_role != 1 {
		// 	response := model.Response{
		// 		Status:  "fatal",
		// 		Message: "У вас нет доступа на эту операцию",
		// 	}

		// 	json.NewEncoder(w).Encode(response)

		// 	return
		// }

		if !token_flag {
			response := model.Response{
				Status:  "fatal",
				Message: "Что-то не так с JWT",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		err = repo.PrintChatSQL(ctx, w, dbpool, user_id)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой PrintChatSQL", op))

			return
		}
	}
}
