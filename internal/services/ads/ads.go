package ads

import (
	"context"
	"encoding/json"
	"fmt"
	"myproject/internal/database"
	"myproject/internal/jwt"
	"myproject/internal/model"
	"net/http"
	"strconv"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gorilla/websocket"

	function "myproject/internal"

	"github.com/go-redis/redis/v8"
	jwt_from_prod_list "github.com/golang-jwt/jwt"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog"
)

func ProductList(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.ads.ProductList"
		var productList model.ProductList

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&productList)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с парсингом JSON-запроса", op))
		}

		repo := database.NewRepo(ctx, dbpool)

		// Попытка прочитать куку
		tokenn, err := r.Cookie("token")
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с попыткой прочитать куку", op))
		}

		// Парсинг и верификация токена
		token, err := jwt_from_prod_list.Parse(tokenn.Value, func(token *jwt_from_prod_list.Token) (interface{}, error) {
			// Проверка метода подписи токена
			if _, ok := token.Method.(*jwt_from_prod_list.SigningMethodHMAC); !ok {
				if err != nil {
					logger.Err(err).Msg(fmt.Sprintf("unexpected signing method: %v", token.Header["alg"]))
				}
				return nil, fmt.Errorf(fmt.Sprintf("unexpected signing method: %v", token.Header["alg"]))
			}
			return "superSecretKey", nil
		})

		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с парсингом и верификации токена", op))
		}

		var user_id *int

		// Извлечение полезной нагрузки (claims) из токена
		if claims, ok := token.Claims.(jwt_from_prod_list.MapClaims); ok {
			// Извлекаем user_id из claims
			if userIDFloat, ok := claims["user_id"].(float64); ok {
				user_id = new(int)          // Создаем указатель
				*user_id = int(userIDFloat) // Записываем значение по указателю
			}
		}

		err = repo.ProductListSQL(ctx, w, dbpool, r, user_id, productList.Ads_list)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой ProductListSQL", op))
		}
	}
}

func PrintAds(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.ads.PrintAds"
		var printAds model.Ads_id

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&printAds)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с парсингом JSON-запроса", op))
		}

		repo := database.NewRepo(ctx, dbpool)

		err = repo.PrintAdsSQL(ctx, w, dbpool, r, printAds.Ads_id)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой PrintAdsSQL", op))
		}
	}
}

func SortProductListDailyRate(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.ads.SortProductListDailyRate"

		var sortProductList model.SortProductListAll

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&sortProductList)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с парсингом JSON-запроса", op))
		}

		// Преобразуем UNIX-время в тип time.Time
		lowDate := time.Unix(sortProductList.LowDate, 0).UTC()
		higDate := time.Unix(sortProductList.HigDate, 0).UTC()

		repo := database.NewRepo(ctx, dbpool)

		err = repo.SortProductListDailyRateSQL(ctx, w, dbpool, r, sortProductList.List, sortProductList.Size, sortProductList.Category, sortProductList.LowNum, sortProductList.HigNum, lowDate, higDate, sortProductList.Position, sortProductList.Location, sortProductList.Distance, sortProductList.Rating)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой SortProductListDailyRateSQL", op))
		}
	}
}

func SortProductListHourlyRate(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.ads.SortProductListHourlyRate"
		var sortProductList model.SortProductListAll

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&sortProductList)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с парсингом JSON-запроса", op))
		}

		// Преобразуем UNIX-время в тип time.Time
		lowDate := time.Unix(sortProductList.LowDate, 0).UTC()
		higDate := time.Unix(sortProductList.HigDate, 0).UTC()

		repo := database.NewRepo(ctx, dbpool)

		err = repo.SortProductListHourlyRateSQL(ctx, w, dbpool, r, sortProductList.List, sortProductList.Size, sortProductList.Category, sortProductList.LowNum, sortProductList.HigNum, lowDate, higDate, sortProductList.Position, sortProductList.Distance, sortProductList.Rating, sortProductList.Location)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой SortProductListHourlyRateSQL", op))
		}
	}
}

func SignupAds(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.ads.SignupAds"
		var ads model.Ads

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&ads)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с парсингом JSON-запроса", op))

			return
		}

		repo := database.NewRepo(ctx, dbpool)

		// Попытка прочитать куку
		token, err := r.Cookie("token")
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с попыткой прочитать куку", op))

			return
		}

		token_flag, user_id, user_role := jwt.IsAuthorized(token.Value)

		var pwd = "/home/beeline/media/ads/"

		err, image_flag, Pwd_mass := function.UploadImagesMass(w, ads.Image, pwd, strconv.Itoa(user_id), logger)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с попыткой сохранить массив медиа", op))
		}

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

		if !image_flag {
			response := model.Response{
				Status:  "fatal",
				Message: "Ошибка с аватаркой обхявления",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		validate := validator.New()
		err = validate.Struct(ads)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с валидацией", op))

			response := model.Response{
				Status:  "fatal",
				Message: "Ошибка с валидацией",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		err = repo.SignupAdsSQL(ctx, w, dbpool, ads.Title, ads.Description, ads.Hourly_rate, ads.Daily_rate, user_id, ads.Category_id, ads.PositionX, ads.PositionY, ads.Location, time.Now(), ads.Image, Pwd_mass, pwd)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой SignupAdsSQL", op))
		}
	}
}

func EditAdsList(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.ads.EditAdsList"
		var ads model.Ads_id

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&ads)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с парсингом JSON-запроса", op))
		}

		repo := database.NewRepo(ctx, dbpool)

		// Попытка прочитать куку
		token, err := r.Cookie("token")
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с попыткой прочитать куку", op))
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
		err = repo.EditAdsListSQL(ctx, w, dbpool, ads.Ads_id, user_id)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой EditAdsListSQL", op))
		}

	}
}

func UpdAds(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.ads.UpdAds"

		var ads model.Upd_ads
		fmt.Println(ads)

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&ads)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с парсингом JSON-запроса", op))
		}

		repo := database.NewRepo(ctx, dbpool)

		// Попытка прочитать куку
		token, err := r.Cookie("token")
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с попыткой прочитать куку", op))
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
		if len(ads.Id_images_from_del) != 0 {
			for i := range len(ads.Id_images_from_del) {
				repo.UpdAdsDelImgSQL(ctx, w, dbpool, ads.Id_images_from_del[i])
			}
		}
		if ads.Title != "" {
			repo.UpdAdsTitleSQL(ctx, w, dbpool, ads.Title, ads.Ads_id, user_id)
		}
		if ads.Description != "" {
			repo.UpdAdsDescriptionSQL(ctx, w, dbpool, ads.Description, ads.Ads_id, user_id)
		}
		if ads.Hourly_rate != 0 {
			repo.UpdAdsHourly_rateSQL(ctx, w, dbpool, ads.Hourly_rate, ads.Ads_id, user_id)
		}
		if ads.Daily_rate != 0 {
			repo.UpdAdsDaily_rateSQL(ctx, w, dbpool, ads.Daily_rate, ads.Ads_id, user_id)
		}
		if ads.Category_id != 0 {
			repo.UpdAdsCategory_idSQL(ctx, w, dbpool, ads.Category_id, ads.Ads_id, user_id)
		}
		if len(ads.Images) != 0 {
			var pwd = "/root/home/beeline_project/media/ads/"
			for i := range len(ads.Images) {
				err, flag, file_path := function.UploadImage(w, ads.Images[i], pwd, strconv.Itoa(user_id), strconv.Itoa(i), logger)
				if err != nil {
					logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с попыткой добавить медиа", op))
				}

				if flag {
					repo.UpdAdsAddImgSQL(ctx, w, dbpool, file_path, ads.Ads_id, user_id)
				}
			}
		}
	}
}

func DelAds(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.ads.DelAds"

		var ads model.Ads_id

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&ads)
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

		err = repo.DelAdsSQL(ctx, w, dbpool, ads.Ads_id, user_id)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой DelAdsSQL", op))
		}
	}
}

func SigFavAds(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool, сonnection map[int]*websocket.Conn) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.ads.SigFavAds"

		var ads model.FavAds

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&ads)
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

		err = repo.SigFavAdsSQL(ctx, dbpool, w, r, redisClient, сonnection, user_id, ads.Ads_id)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой SigFavAdsSQL", op))
		}
	}
}

func DelFavAds(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.ads.DelFavAds"

		var ads model.FavAds

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&ads)
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

		err = repo.DelFavAdsSQL(ctx, dbpool, w, user_id, ads.Ads_id)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой DelFavAdsSQL", op))
		}
	}
}

func SearchForTech(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.ads.SearchForTech"

		var title model.Ads

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&title)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с парсингом JSON-запроса", op))
		}

		repo := database.NewRepo(ctx, dbpool)

		err = repo.SearchForTechSQL(ctx, title.Title, w, dbpool)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой SearchForTechSQL", op))
		}
	}
}

func SortProductListCategoriez(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.ads.SortProductListCategoriez"

		var sortProductList model.SortProductListAll

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&sortProductList)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с парсингом JSON-запроса", op))
		}

		repo := database.NewRepo(ctx, dbpool)

		err = repo.SortProductListCategoriezSQL(ctx, w, dbpool, r, sortProductList.Category)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой SortProductListCategoriezSQL", op))
		}
	}
}

func GroupAdsByHourlyRate(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.ads.GroupAdsByHourlyRate"

		repo := database.NewRepo(ctx, dbpool)

		err := repo.GroupAdsByHourlyRateSQL(ctx, w, dbpool)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой GroupAdsByHourlyRateSQL", op))
		}
	}
}

func GroupAdsByDailyRate(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.ads.GroupAdsByDailyRate"

		repo := database.NewRepo(ctx, dbpool)

		err := repo.GroupAdsByDailyRateSQL(ctx, w, dbpool)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой GroupAdsByDailyRateSQL", op))
		}
	}
}

func GroupFavByRecent(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.ads.GroupFavByRecent"

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

		err = repo.GroupFavByRecentSQL(ctx, w, dbpool, r, user_id)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой GroupFavByRecentSQL", op))
		}
	}
}

func GroupFavByCheaper(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.ads.GroupFavByCheaper"

		repo := database.NewRepo(ctx, dbpool)

		token, err := jwt.ReadCookie("token", r)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с попыткой прочитать куку", op))
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
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с попыткой прочитать JWT", op))

			response := model.Response{
				Status:  "fatal",
				Message: "Что-то не так с JWT",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		err = repo.GroupFavByCheaperSQL(ctx, w, dbpool, r, user_id)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой GroupFavByCheaperSQL", op))
		}
	}
}

func GroupFavByDearly(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.ads.GroupFavByDearly"

		repo := database.NewRepo(ctx, dbpool)

		token, err := jwt.ReadCookie("token", r)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с попыткой прочитать куку", op))
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
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с попыткой прочитать JWT", op))

			response := model.Response{
				Status:  "fatal",
				Message: "Что-то не так с JWT",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		err = repo.GroupFavByDearlySQL(ctx, w, dbpool, r, user_id)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой GroupFavByDearlySQL", op))
		}
	}
}

func GroupAdsByRented(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.ads.GroupAdsByRented"

		repo := database.NewRepo(ctx, dbpool)

		token, err := jwt.ReadCookie("token", r)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с попыткой прочитать куку", op))
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
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с попыткой прочитать JWT", op))

			response := model.Response{
				Status:  "fatal",
				Message: "Что-то не так с JWT",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		err = repo.GroupAdsByRentedSQL(ctx, w, dbpool, user_id)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой GroupAdsByRentedSQL", op))
		}
	}
}

func GroupAdsByArchived(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.ads.GroupAdsByArchived"

		repo := database.NewRepo(ctx, dbpool)

		token, err := jwt.ReadCookie("token", r)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с попыткой прочитать куку", op))
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
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с попыткой прочитать JWT", op))

			response := model.Response{
				Status:  "fatal",
				Message: "Что-то не так с JWT",
			}

			json.NewEncoder(w).Encode(response)

			return
		}

		err = repo.GroupAdsByArchivedSQL(ctx, w, user_id, dbpool)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой GroupAdsByArchivedSQL", op))
		}
	}
}

func AllUserAds(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.ads.AllUserAds"

		var owner_id model.Owner_id

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&owner_id)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с парсингом JSON-запроса", op))
		}

		repo := database.NewRepo(ctx, dbpool)

		err = repo.AllUserAdsSQL(ctx, w, dbpool, r, owner_id.Owner_id)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой AllUserAdsSQL", op))
		}
	}
}

func AllAdsOfThisUser(redisClient *redis.Client, logger zerolog.Logger, ctx context.Context, dbpool *pgxpool.Pool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		op := "internal.services.ads.AllAdsOfThisUser"

		var owner_id model.Owner_id

		// Парсинг JSON-запроса
		err := json.NewDecoder(r.Body).Decode(&owner_id)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с парсингом JSON-запроса", op))
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
		err = repo.AllAdsOfThisUserSQL(ctx, w, dbpool, r, user_id, owner_id.Owner_id)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка с процедурой AllAdsOfThisUserSQL", op))
		}
	}
}
