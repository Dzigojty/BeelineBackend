package test

// import (
// 	"bytes"
// 	"context"
// 	"encoding/json"
// 	"fmt"
// 	"myproject/internal/services/user"
// 	"net/http"
// 	"net/http/httptest"
// 	"os"
// 	"testing"

// 	"github.com/go-redis/redis/v8"
// 	"github.com/julienschmidt/httprouter"
// 	"github.com/rs/zerolog"
// 	"github.com/stretchr/testify/require"
// )

// func TestEnterCodeHandler(t *testing.T) {
// 	op := "internal.services.user.test.TestSignupUserHandler"

// 	// Подключение к реальному Redis
// 	redisClient := redis.NewClient(&redis.Options{
// 		Addr: "localhost:6379", // Убедитесь, что Redis работает локально или измените адрес
// 	})
// 	defer redisClient.Close()

// 	ctx := context.Background()
// 	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr})

// 	handlerData := &user.SignupHandler{
// 		RedisClient: redisClient,
// 		Logger:      logger,
// 	}

// 	cases := []struct { // описание кейса для тестов
// 		name         string // имя теста, в котором содержится его кратакя характеристика
// 		emailOrPhone string // почта или телефон, который указывают и на который придёт код
// 		typee        int    // это нужно для тестов (1 - регистрация через имейл, 2 - через номер телефона)
// 		expectCode   int    // код из ответа
// 		expectMsg    string // JSON ответ
// 		respError    string // это поле содержит ожидаемую ошибку, которую хендлер должен вернуть в JSON-ответе, если что-то пошло не так.
// 		// симулируя ошибочное поведение зависимостей. Оно используется для проверки, как хендлер обрабатывает внутренние ошибки.
// 	}{
// 		{ // кейс где все ок
// 			name:         "Valid email",
// 			emailOrPhone: "test.octa.one@gmail.com",
// 			typee:        1,
// 			expectMsg:    "Почта принята",
// 		},
// 		// { // кейс где все ок
// 		// 	name:         "Valid phone",
// 		// 	emailOrPhone: "+7(928)074-32-44",
// 		// 	typee:        2,
// 		// 	expectMsg: "Телефон принят",
// 		// },
// 		{ // кейс где указана несуществующая почта
// 			name:         "Non-existent email",
// 			emailOrPhone: "test.octa.azamat@gmail.com",
// 			typee:        1,
// 			expectMsg:    "Почты получателя не существует",
// 		},
// 		// { // кейс где указан несуществующий номер
// 		// 	name:         "Non-existent phone",
// 		// 	emailOrPhone: "+7(777)777-77-77",
// 		// 	typee:        2,
// 		// },
// 		{ //кейс где указан неверный формат почты (нет такого оператора)
// 			name:         "Invalid email format",
// 			emailOrPhone: "test.octa.one@gagarin.com",
// 			typee:        1,
// 			expectMsg:    "Почта принята",
// 		},
// 		{ //кейс где указан неверный формат почты (нет такого оператора)
// 			name:         "Invalid email format",
// 			emailOrPhone: "test.octa.one@gagagarin.com",
// 			typee:        1,
// 			expectMsg:    "Почты получателя не существует",
// 		},
// 		// { //кейс где указан неверный формат телефона (слишком много символов)
// 		// 	name:         "Invalid phone format",
// 		// 	emailOrPhone: "+7(777)777-777-777",
// 		// 	typee:        2,
// 		// },
// 		{ //кейс где указан неверный формат почты (недопустимые символы)
// 			name:         "Invalid email symbol",
// 			emailOrPhone: "test_octa_one@gmail.com",
// 			typee:        1,
// 			expectMsg:    "Почты получателя не существует",
// 		},
// 		// { //кейс где указан неверный формат телефона (недопустимые символы)
// 		// 	name:         "Invalid phone symbol",
// 		// 	emailOrPhone: "+7(928)074_32_44",
// 		// 	typee:        2,
// 		// },
// 		{ // кейс где слишком много символов в почте
// 			name:         "Too much email symbol",
// 			emailOrPhone: "test.octa.oneeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee@gmail.com",
// 			typee:        1,
// 			expectMsg:    "Почты получателя не существует",
// 		},
// 		{ // пустое значение
// 			name:         "Empty email",
// 			emailOrPhone: "",
// 			typee:        1,
// 			expectMsg:    "Почты получателя не существует",
// 		},
// 		// { // пустое значение
// 		// 	name:         "Empty phone",
// 		// 	emailOrPhone: "",
// 		// 	typee:        2,
// 		// },
// 		{ // странный символ
// 			name:         "Incorrect email symbol",
// 			emailOrPhone: "Ъ",
// 			typee:        1,
// 			expectMsg:    "Почты получателя не существует",
// 		},
// 		// { // странный символ
// 		// 	name:         "Incorrect phone symbol",
// 		// 	emailOrPhone: "+",
// 		// 	typee:        2,
// 		// },
// 		{ // странный символ
// 			name:         "Strange email symbol 1",
// 			emailOrPhone: "0",
// 			typee:        1,
// 			expectMsg:    "Почты получателя не существует",
// 		},
// 		// { // странный символ
// 		// 	name:         "Strange phone symbol 1",
// 		// 	emailOrPhone: "Ъ",
// 		// 	typee:        2,
// 		// },
// 		{ // странный символ
// 			name:         "Strange email symbol 2",
// 			emailOrPhone: "._+",
// 			typee:        1,
// 			expectMsg:    "Почты получателя не существует",
// 		},
// 		// { // странный символ
// 		// 	name:         "Strange phone symbol 2",
// 		// 	emailOrPhone: "._+",
// 		// 	typee:        2,
// 		// },
// 	}

// 	for _, tc := range cases { // запускаем цикл, который бы прогнал тест кейсы
// 		t.Run(tc.name, func(t *testing.T) { // запуск теста, где учитывается его имя и выполняется функция
// 			handler := handlerData.SignupUserByEmail(redisClient, logger, ctx) // сам хендлер
// 			wrappedHandler := httprouterAdapter(handler)                       // обёртка, чтобы он возвращал (http.Handler)

// 			payload := fmt.Sprintf(`{"Email": "%s"}`, tc.emailOrPhone)                                          // тестовый JSON запрос
// 			req := httptest.NewRequest(http.MethodPost, "/signupUserByEmail", bytes.NewBuffer([]byte(payload))) // тестовый HTTP-запрос
// 			rr := httptest.NewRecorder()                                                                        // создание объекта для записи ответа

// 			wrappedHandler.ServeHTTP(rr, req) // вызов хендлера
// 			// Проверяем тело ответа
// 			var resp map[string]interface{}                            // карта для распарсенных данных ответа
// 			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp)) // получает тело HTTP-ответа
// 			require.Contains(t, resp["message"], tc.expectMsg)         // сверяет сообщения, заложенные в тестах и полученные сорваком

// 			if resp["data"] != nil {
// 				val, err := redisClient.Get(ctx, resp["data"].(string)).Result()
// 				if err != nil {
// 					logger.Err(err).Msg(fmt.Sprintf("error in %s", op+"; Ошибка при от парсинге данных, полученных из redis"))

// 					return
// 				}

// 				type Kesh struct {
// 					Email string `json:"Email"`
// 					Code  int    `json:"Code"`
// 				}

// 				// Создаем экземпляр структуры Kesh
// 				var kesh Kesh

// 				// Распарсим строку из Redis в структуру Kesh
// 				err = json.Unmarshal([]byte(val), &kesh)
// 				if err != nil {
// 					logger.Err(err).Msg(fmt.Sprintf("error in %s", op+"; Ошибка при парсинге JSON из данных Redis"))
// 					return
// 				}

// 				require.Equal(t, kesh.Code, handlerData.CodeNum) // проверка кода ответа
// 			}

// 			require.Equal(t, http.StatusOK, rr.Code) // проверка кода ответа
// 		})
// 	}
// }

// /*
// 	fmt.Println("Info: ", resp)
// 	fmt.Println("Data: ", resp["data"])
// 	fmt.Println("Message: ", resp["message"])
// */

// func httprouterAdapter(h httprouter.Handle) http.Handler {
// 	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		h(w, r, httprouter.Params{})
// 	})
// }
