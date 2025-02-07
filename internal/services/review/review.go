package review

import (
	"context"
	"encoding/json"
	"fmt"
	"myproject/internal/database"
	"myproject/internal/jwt"
	"myproject/internal/model"
	"net/http"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog"
)

func SigReview(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool, сonnections map[int]*websocket.Conn) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.review.SigReview"

		var sigReview model.SigReview

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&sigReview)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in ", op, "; ошибка с парсингом JSON-запроса"))
		}

		repo := database.NewRepo(ctx, dbpool)

		// Попытка прочитать куку
		token, err := r.Cookie("token")
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in ", op, "; ошибка с попыткой прочитать куку"))
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
		err = repo.SigReviewSQL(ctx, w, dbpool, r, redisClient, сonnections, user_id, sigReview.Ads_id, sigReview.Rating, sigReview.Comment, sigReview.State)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой SigReviewSQL", op))
		}
	}
}

func UpdReview(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.review.UpdReview"

		var updReview model.UpdReview

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&updReview)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in ", op, "; ошибка с парсингом JSON-запроса"))
		}

		repo := database.NewRepo(ctx, dbpool)

		// Попытка прочитать куку
		token, err := r.Cookie("token")
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in ", op, "; ошибка с попыткой прочитать куку"))
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

		err = repo.UpdReviewSQL(ctx, w, dbpool, user_id, updReview.Review_id, updReview.Rating, updReview.Comment)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s", op, "; ошибка с процедурой UpdReviewSQL"))
		}
	}
}

func GroupReviewNewOnesFirst(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.review.GroupReviewNewOnesFirst"

		var ads_id model.Ads_id

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&ads_id)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in ", op, "; ошибка с парсингом JSON-запроса"))
		}

		repo := database.NewRepo(ctx, dbpool)

		err = repo.GroupReviewNewOnesFirstSQL(ctx, w, dbpool, r, ads_id.Ads_id)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s", op, "; ошибка с процедурой GroupReviewNewOnesFirstSQL"))
		}
	}
}

func GroupReviewOldOnesFirst(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.review.GroupReviewOldOnesFirst"

		var ads_id model.Ads_id

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&ads_id)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in ", op, "; ошибка с парсингом JSON-запроса"))
		}

		repo := database.NewRepo(ctx, dbpool)

		err = repo.GroupReviewOldOnesFirstSQL(ctx, w, dbpool, r, ads_id.Ads_id)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s", op, "; ошибка с процедурой GroupReviewOldOnesFirstSQL"))
		}
	}
}

func GroupReviewLowRatOnesFirst(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.review.GroupReviewLowRatOnesFirst"

		var ads_id model.Ads_id

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&ads_id)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in ", op, "; ошибка с парсингом JSON-запроса"))
		}

		repo := database.NewRepo(ctx, dbpool)

		err = repo.GroupReviewLowRatOnesFirstSQL(ctx, w, dbpool, r, ads_id.Ads_id)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s", op, "; ошибка с процедурой GroupReviewLowRatOnesFirstSQL"))
		}
	}
}

func GroupReviewHigRatOnesFirst(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.review.GroupReviewHigRatOnesFirst"

		var ads_id model.Ads_id

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&ads_id)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in ", op, "; ошибка с парсингом JSON-запроса"))
		}

		repo := database.NewRepo(ctx, dbpool)

		err = repo.GroupReviewHigRatOnesFirstSQL(ctx, w, dbpool, r, ads_id.Ads_id)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s", op, "; ошибка с процедурой GroupReviewHigRatOnesFirstSQL"))
		}
	}
}
