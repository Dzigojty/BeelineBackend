package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"sync"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog"

	"myproject/internal/jwt"
	"myproject/internal/model"
	"myproject/internal/services/ads"
	"myproject/internal/services/chat"
	"myproject/internal/services/finance"
	"myproject/internal/services/login"
	"myproject/internal/services/mediator"
	"myproject/internal/services/order"
	"myproject/internal/services/review"
	"myproject/internal/services/user"
)

type MyApp struct {
	app model.App
}

// Настройка апгрейдера для преобразования HTTP-соединений в WebSocket
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // В реальных проектах проверьте домен клиента
	},
}

var connections = make(map[int]*websocket.Conn)
var mu sync.Mutex // Для синхронизации доступа к карте

func handleWebSocket(w http.ResponseWriter, r *http.Request, redisClient *redis.Client, userID int) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Error upgrading to websocket:", err)
		return
	}

	mu.Lock()
	connections[userID] = conn
	mu.Unlock()
	log.Printf("User %d connected", userID)

	fmt.Println(strconv.Itoa(userID))

	//проверка кеша и отправка, если там что-то есть
	// Проверяем, есть ли данные в Redis
	// Чтение данных из Redis как строки
	messages, err := redisClient.LRange(context.Background(), strconv.Itoa(userID), 0, -1).Result()
	if err != nil {
		fmt.Println("Ошибка Redis:", err)
		return
	}

	// Десериализуем уведомления и отправляем их клиенту
	for _, message := range messages {
		var notif model.NotifType
		err := json.Unmarshal([]byte(message), &notif)
		if err != nil {
			fmt.Println("Ошибка десериализации:", err)
			continue
		}
		// Отправляем данные клиенту (например, через WebSocket)
		conn.WriteMessage(websocket.TextMessage, []byte(message))
	}

	// Удаляем отправленные данные из очереди
	redisClient.Del(context.Background(), strconv.Itoa(userID))
}

func NewRepository(pool *pgxpool.Pool) *model.Repository {
	return &model.Repository{Pool: pool}
}

func NewApp(Ctx context.Context, dbpool *pgxpool.Pool) *MyApp {
	return &MyApp{model.App{Ctx: Ctx, Repo: NewRepository(dbpool), Cache: make(map[string]model.User)}}
}

func StartPage(rw http.ResponseWriter, r *http.Request, p httprouter.Params) {
	fmt.Fprintf(rw, "")
}

func handleTextMessage() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
			return
		}
		defer r.Body.Close()

		fmt.Printf("Received message: %s\n", string(body))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Message received"))
	}
}

func (application *MyApp) Routes(r *httprouter.Router, Ctx context.Context, dbpool *pgxpool.Pool, rdb *redis.Client, logger zerolog.Logger) {
	//WebSocket-соединения
	r.GET("/handleWebSocket", func(rw http.ResponseWriter, r *http.Request, p httprouter.Params) {
		// Извлечение токена JWT из query параметра
		token := r.URL.Query().Get("token")

		flag, user_id, _ := jwt.IsAuthorized(token)
		if flag {
			handleWebSocket(rw, r, rdb, user_id)
			// сonnections = make(map[int]*websocket.Conn) // ключ — userID, значение — соединение

			return
		}

		response := model.Response{
			Status:  "success",
			Message: "Показано",
		}

		json.NewEncoder(rw).Encode(response)

		return
	})

	r.GET("/disconnWebSocket", func(rw http.ResponseWriter, r *http.Request, p httprouter.Params) {
		// Извлечение токена JWT из query параметра
		token, err := jwt.ReadCookie("token", r)
		if err != nil {
			fmt.Println(err)
		}

		flag, user_id, _ := jwt.IsAuthorized(token)
		if flag {
			delete(connections, user_id)
			if connections[user_id] == nil {

				response := model.Response{
					Status:  "success",
					Message: "Показано",
				}

				json.NewEncoder(rw).Encode(response)

				return
			}
		}

		response := model.Response{
			Status:  "fatal",
			Message: "не удалось разоарвать соединение",
		}

		json.NewEncoder(rw).Encode(response)

		return
	})
	r.ServeFiles("/public/*filepath", http.Dir("public"))

	user_handler := &user.SignupHandler{
		RedisClient: rdb,
		Logger:      logger,
	}

	r.POST("/message", handleTextMessage()) // это нужно удалить в проде

	// схема user
	// signupUser_test
	r.POST("/signupUserByEmail", user.SignupUserByEmailCreater(rdb, logger, Ctx)) //пользователь укзывает почту(регистрация)
	r.POST("/signupUserByPhone", user.SignupUserByPhoneCreater(rdb, logger, Ctx)) //пользователь укзывает телефон(регистрация)
	// enterCode_test
	r.POST("/enterCodeFromEmail", user.EnterCodeFromEmailCreater(rdb, logger, Ctx)) //пользователь укзывает код почта
	r.POST("/enterCodeFromPhone", user.EnterCodeFromPhoneCreater(rdb, logger, Ctx)) //пользователь укзывает код телефон
	// signupLegal_test
	r.POST("/signupLegalEmail", user.SignupLegalEmailCreater(rdb, logger, Ctx, dbpool)) //передача данных Юридического лица (регистрация) Email
	r.POST("/signupLegalPhone", user.SignupLegalPhoneCreater(rdb, logger, Ctx, dbpool)) //передача данных Юридического лица (регистрация) Photo
	// signupNatur_test
	r.POST("/signupNaturEmail", user.SignupNaturEmail(rdb, logger, Ctx, dbpool)) //передача данных Физического лица (регистрация) Email
	r.POST("/signupNaturPhone", user.SignupNaturPhone(rdb, logger, Ctx, dbpool)) //передача данных Физического лица (регистрация) Email
	// editingUser_test
	r.POST("/editingLegalUserData", user.EditingLegalUserData(rdb, logger, Ctx, dbpool)) //изменение данных для юрика
	r.POST("/editingNaturUserData", user.EditingNaturUserData(rdb, logger, Ctx, dbpool)) //изменение данных для физика
	// sendCod_test
	r.POST("/sendCodForEmail", user.SendCodForEmail(rdb, logger, Ctx))               //отправка сообщения на почту для подтверждения
	r.POST("/sendCodForPhoneNum", user_handler.SendCodForPhoneNum(rdb, logger, Ctx)) //отправка сообщения на телефон для подтверждения
	// enterCod_test
	r.POST("/enterCodFromEmail", user.EnterCodFromEmail(rdb, logger, Ctx, dbpool))       //отправка сообщения на почту для подтверждения
	r.POST("/enterCodFromPhoneNum", user.EnterCodFromPhoneNum(rdb, logger, Ctx, dbpool)) //отправка сообщения на телефон для подтверждения
	// favProfilGroup_test
	r.GET("/favProfilsFirstNew", user.FavProfilsFirstNew(rdb, logger, Ctx, dbpool))     //групировка профиля
	r.GET("/favProfilsFirstOld", user.FavProfilsFirstOld(rdb, logger, Ctx, dbpool))     //групировка профиля
	r.GET("/favProfilsFirstCheap", user.FavProfilsFirstCheap(rdb, logger, Ctx, dbpool)) //групировка профиля
	r.GET("/favProfilsFirstDearl", user.FavProfilsFirstDearl(rdb, logger, Ctx, dbpool)) //групировка профиля

	// схема login
	r.POST("/loginYandex", login.LoginYandex(rdb, logger, Ctx, dbpool))                                                   // Это используется при нажатии кнопки "Авторизироватьяс через Яндекс"
	r.POST("/callback", login.Callback(rdb, logger, Ctx, dbpool))                                                         // Обработка обратного вызова авторизации через Яндекс, если она прошла успешно
	r.POST("/login", login.Login(rdb, logger, Ctx, dbpool))                                                               //логин отправка
	r.POST("/RecoveryPasswdEmail", login.RecoveryPasswdEmail(rdb, logger, Ctx, dbpool))                                   //восстановление пароля
	r.POST("/recoveryPasswdPhone", login.RecoveryPasswdPhone(rdb, logger, Ctx, dbpool))                                   //восстановление пароля
	r.POST("/enterCodeForRecoveryPassWithEmail", login.EnterCodeForRecoveryPassWithEmail(rdb, logger, Ctx, dbpool))       //восстановление пароля через почту(отправление на почту)
	r.POST("/sendCodeForRecoveryPassWithEmail", login.SendCodeForRecoveryPassWithEmail(rdb, logger, Ctx, dbpool))         //восстановление пароля через почту
	r.POST("/enterCodeForRecoveryPassWithPhoneNum", login.EnterCodeForRecoveryPassWithPhoneNum(rdb, logger, Ctx, dbpool)) //восстановление пароля через телефон
	r.POST("/sendCodeForRecoveryPassWithPhoneNum", login.SendCodeForRecoveryPassWithPhoneNum(rdb, logger, Ctx, dbpool))   //восстановление пароля через телефон
	r.POST("/recoveryPass", login.RecoveryPass(rdb, logger, Ctx, dbpool))                                                 //авторизованное восстановление пароля через телефон
	r.POST("/autorizLoginEmailSend", login.AutorizLoginEmailSend(rdb, logger, Ctx, dbpool))                               //логин отправка
	r.POST("/autorizLoginEmailEnter", login.AutorizLoginEmailEnter(rdb, logger, Ctx, dbpool))                             //логин ввод
	r.GET("/refreshToken", login.RefreshToken(rdb, logger, Ctx, dbpool))                                                  //рефреш токены
	r.POST("/sendCode", login.SendCode(rdb, logger, Ctx, dbpool))                                                         //вводим код
	r.POST("/enterPasswd", login.EnterPasswd(rdb, logger, Ctx, dbpool))                                                   //вводим код
	r.POST("/addAddress", login.AddAddress(rdb, logger, Ctx, dbpool))                                                     // добавляем адрес
	r.GET("/giveAddress", login.GiveAddress(rdb, logger, Ctx, dbpool))                                                    // смотрим

	// схема ads
	r.POST("/productList", ads.ProductList(rdb, logger, Ctx, dbpool))                             //
	r.POST("/printAds", ads.PrintAds(rdb, logger, Ctx, dbpool))                                   //вывод продукта
	r.POST("/sortProductListDailyRate", ads.SortProductListDailyRate(rdb, logger, Ctx, dbpool))   //вывод продукта с учётом сортировки всем категориям
	r.POST("/sortProductListHourlyRate", ads.SortProductListHourlyRate(rdb, logger, Ctx, dbpool)) //вывод продукта с учётом сортировки всем категориям
	r.POST("/sigAds", ads.SignupAds(rdb, logger, Ctx, dbpool))                                    //размещение(добавление) объявления
	r.POST("/editAdsList", ads.EditAdsList(rdb, logger, Ctx, dbpool))                             //редактирование(изменение) объявления
	r.POST("/updAds", ads.UpdAds(rdb, logger, Ctx, dbpool))                                       //редактирование(изменение) объявления
	r.POST("/delAds", ads.DelAds(rdb, logger, Ctx, dbpool))                                       //удаление объявления
	r.POST("/sigFavAds", ads.SigFavAds(rdb, logger, Ctx, dbpool, connections))                    //добавление объявления в избранное
	r.POST("/delFavAds", ads.DelFavAds(rdb, logger, Ctx, dbpool))                                 //удаление объявления из избранного
	r.POST("/searchForTech", ads.SearchForTech(rdb, logger, Ctx, dbpool))                         //поиск объявления
	r.POST("/sortProductListCategoriez", ads.SortProductListCategoriez(rdb, logger, Ctx, dbpool)) //вывод продукта с учётом сортировки категории
	r.GET("/groupAdsByHourlyRate", ads.GroupAdsByHourlyRate(rdb, logger, Ctx, dbpool))            //группировка объявлений, сначала дороже(почасовая цена)
	r.GET("/groupFavByRecent", ads.GroupFavByRecent(rdb, logger, Ctx, dbpool))                    //группировка избранных объявлений, сначала новые
	r.GET("/groupFavByCheaper", ads.GroupFavByCheaper(rdb, logger, Ctx, dbpool))                  //группировка избранных объявлений, сначала дороже
	r.GET("/groupFavByDearly", ads.GroupFavByDearly(rdb, logger, Ctx, dbpool))                    //группировка избранных объявлений, сначала дешевле
	r.GET("/groupAdsByRented", ads.GroupAdsByRented(rdb, logger, Ctx, dbpool))                    //вывод объявлений по хозяину(активных)
	r.GET("/groupAdsByArchived", ads.GroupAdsByArchived(rdb, logger, Ctx, dbpool))                //вывод объявлений по хозяину(неактивный)
	r.POST("/allUserAds", ads.AllUserAds(rdb, logger, Ctx, dbpool))                               //Кнопка 11 объявлений пользователя
	r.POST("/allAdsOfThisUser", ads.AllAdsOfThisUser(rdb, logger, Ctx, dbpool))                   //все объявления этого юзера

	// схема chat
	r.POST("/chatButtonInAds", chat.ChatButtonInAds(rdb, logger, Ctx, dbpool))                      //кнопка "написать" в листе объявления
	r.POST("/sigChat", chat.SigChat(rdb, logger, Ctx, dbpool))                                      //начало переписки
	r.POST("/openChat", chat.OpenChat(rdb, logger, Ctx, dbpool))                                    //открытие чата
	r.POST("/sendMessageAndImage", chat.SendMessageAndImage(rdb, logger, Ctx, dbpool, connections)) //отправить сообщение и медиа
	r.POST("/sendImage", chat.SendImage(rdb, logger, Ctx, dbpool, connections))                     //отправить медиа
	r.POST("/sendMessage", chat.SendMessage(rdb, logger, Ctx, dbpool, connections))                 //отправить сообщение
	r.POST("/sendMessageAndVideo", chat.SendMessageAndVideo(rdb, logger, Ctx, dbpool, connections)) //отправить сообщение и медиа
	r.POST("/sendVideo", chat.SendVideo(rdb, logger, Ctx, dbpool, connections))                     //отправить сообщение
	r.POST("/sigDisputInChat", chat.SigDisputInChat(rdb, logger, Ctx, dbpool))                      //начать спор
	r.GET("/printChat", chat.PrintChat(rdb, logger, Ctx, dbpool))                                   //вывод всех чатов

	// схема review
	r.POST("/sigReview", review.SigReview(rdb, logger, Ctx, dbpool, connections))                      //оставить отзыв
	r.POST("/updReview", review.UpdReview(rdb, logger, Ctx, dbpool))                                   //обновить отзыв
	r.POST("/groupReviewNewOnesFirst", review.GroupReviewNewOnesFirst(rdb, logger, Ctx, dbpool))       //вывод отзывов по порядку, сначала новые
	r.POST("/groupReviewOldOnesFirst", review.GroupReviewOldOnesFirst(rdb, logger, Ctx, dbpool))       //вывод отзывов не по порядку, сначала старые
	r.POST("/groupReviewLowRatOnesFirst", review.GroupReviewLowRatOnesFirst(rdb, logger, Ctx, dbpool)) //вывод отзывов, сначала с высокой оценкой
	r.POST("/groupReviewHigRatOnesFirst", review.GroupReviewHigRatOnesFirst(rdb, logger, Ctx, dbpool)) //вывод отзывов, сначала с низкой оценкой

	// схема mediator
	r.GET("/disputeChatPanel", mediator.DisputeChatPanel(rdb, logger, Ctx, dbpool))              //показать лист спорных чатов
	r.POST("/mediatorEnterInChat", mediator.MediatorEnterInChat(rdb, logger, Ctx, dbpool))       //принять спор на себя(работа медиатора)
	r.POST("/rebookList", mediator.RebookList(rdb, logger, Ctx, dbpool))                         // по какому бронированию у нас спор (rebookList)
	r.POST("/regReport", mediator.RegReport(rdb, logger, Ctx, dbpool))                           //регистрация репорта
	r.POST("/mediatorFinishJobUser", mediator.MediatorFinishJobUser(rdb, logger, Ctx, dbpool))   //медиатор выносит решение
	r.POST("/mediatorFinishJobOwner", mediator.MediatorFinishJobOwner(rdb, logger, Ctx, dbpool)) //медиатор выносит решение
	// r.POST("/printReport", mediator.PrintReportPOST(rw, r, logger)) //вывод репорта

	// схема finance
	r.POST("/transactionToAnother", finance.TransactionToAnother(rdb, logger, Ctx, dbpool))           //получаем от юзера деньги(или отправляем ему их)
	r.POST("/transactionToSomething", finance.TransactionToSomething(rdb, logger, Ctx, dbpool))       //получаем возврат денег от системы(возврат по ошибке или что-то похожее)
	r.POST("/transactionToReturnAmount", finance.TransactionToReturnAmount(rdb, logger, Ctx, dbpool)) //платим деньги за что-то системе(не конкретному юзеру)
	r.POST("/walletHistory", finance.WalletHistory(rdb, logger, Ctx, dbpool))                         //история кошелька
	r.GET("/walletList", finance.WalletList(rdb, logger, Ctx, dbpool))                                //лист кошелька

	// схема order
	r.POST("/regOrderHourly", order.RegOrderHourly(rdb, logger, Ctx, dbpool, connections))   //броинрование и оформление заказа
	r.POST("/regOrderDaily", order.RegOrderDaily(rdb, logger, Ctx, dbpool, connections))     //броинрование и оформление заказа
	r.POST("/regOrderBidding", order.RegOrderBidding(rdb, logger, Ctx, dbpool, connections)) //броинрование на основе торгов
	r.POST("/rebookOrder", order.RebookOrder(rdb, logger, Ctx, dbpool))                      //переброинрование TYT
	r.GET("/groupOrdersByRented", order.GroupOrdersByRented(rdb, logger, Ctx, dbpool))       //группировка заказов по активным
	r.GET("/groupOrdersByUnRented", order.GroupOrdersByUnRented(rdb, logger, Ctx, dbpool))   //группировка заказов по неактивным
	r.POST("/complBooking", order.ComplBooking(rdb, logger, Ctx, dbpool))                    //бронирование прошло успешно и мы начисляем бабки юзеру
	r.GET("/bookingList", order.BookingList(rdb, logger, Ctx, dbpool))                       // лист бронирования
	r.GET("/sigPDFfile", order.SigPDFfile(rdb, logger, Ctx, dbpool))                         // лист бронирования
}
