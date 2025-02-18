package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"myproject/internal/services/user"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"testing"

	"github.com/go-redis/redismock/v8"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

// FlashCallClient описывает, как мы работаем со "zvonok" API
type FlashCallClient interface {
	DoFlashCall(ctx context.Context, phone string, code int) ([]byte, error)
}

func TestSignupUserHandler(t *testing.T) {
	// op := "internal.services.user.test.TestSignupUserHandler"

	cases := []struct { // описание кейса для тестов
		name         string // имя теста, в котором содержится его кратакя характеристика
		emailOrPhone string // почта или телефон, который указывают и на который придёт код
		typee        int    // это нужно для тестов (1 - регистрация через имейл, 2 - через номер телефона)
		expectCode   int    // код из ответа
		expectMsg    string // JSON ответ
		respError    string // это поле содержит ожидаемую ошибку, которую хендлер должен вернуть в JSON-ответе, если что-то пошло не так.
		// симулируя ошибочное поведение зависимостей. Оно используется для проверки, как хендлер обрабатывает внутренние ошибки.
	}{
		{ // кейс где все ок
			name:         "Valid email",
			emailOrPhone: "test.octa.one@gmail.com",
			typee:        1,
			expectMsg:    "Почта принята",
		},
		{ // кейс где все ок
			name:         "Valid phone",
			emailOrPhone: "+7(928)074-32-44",
			typee:        2,
			expectMsg:    "Телефон принят",
		},
		{ // кейс где указана несуществующая почта
			name:         "Non-existent email",
			emailOrPhone: "test.octa.azamat@gmail.com",
			typee:        1,
			expectMsg:    "Почты получателя не существует",
			respError:    "почты test.octa.azamat@gmail.com не существует",
		},
		{ // кейс где указан несуществующий номер
			name:         "Non-existent phone",
			emailOrPhone: "+7(777)777-77-77",
			typee:        2,
			expectMsg:    "Телефон принят",
		},
		{ //кейс где указан неверный формат почты (нет такого оператора)
			name:         "Invalid email format",
			emailOrPhone: "test.octa.one@gagarin.com",
			typee:        1,
			expectMsg:    "Почта принята",
			respError:    "почты test.octa.one@gagagarin.com не существует",
		},
		{ //кейс где указан неверный формат почты (нет такого оператора)
			name:         "Invalid email format",
			emailOrPhone: "test.octa.one@gagagarin.com",
			typee:        1,
			expectMsg:    "Почты получателя не существует",
		},
		{ //кейс где указан неверный формат телефона (слишком много символов)
			name:         "Invalid phone format",
			emailOrPhone: "+7(777)777-777-777",
			typee:        2,
			expectMsg:    "Телефон принят",
		},
		{ //кейс где указан неверный формат почты (недопустимые символы)
			name:         "Invalid email symbol",
			emailOrPhone: "test_octa_one@gmail.com",
			typee:        1,
			expectMsg:    "Почты получателя не существует",
		},
		{ //кейс где указан неверный формат телефона (недопустимые символы)
			name:         "Invalid phone symbol",
			emailOrPhone: "+7(928)074_32_44",
			typee:        2,
			expectMsg:    "Телефон принят",
		},
		{ // кейс где слишком много символов в почте
			name:         "Too much email symbol",
			emailOrPhone: "test.octa.oneeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee@gmail.com",
			typee:        1,
			expectMsg:    "Почты получателя не существует",
		},
		{ // пустое значение
			name:         "Empty email",
			emailOrPhone: "",
			typee:        1,
			expectMsg:    "Почты получателя не существует",
		},
		{ // пустое значение
			name:         "Empty phone",
			emailOrPhone: "",
			typee:        2,
			expectMsg:    "Телефон принят",
		},
		{ // странный символ
			name:         "Incorrect email symbol",
			emailOrPhone: "Ъ",
			typee:        1,
			expectMsg:    "Почты получателя не существует",
		},
		{ // странный символ
			name:         "Incorrect phone symbol",
			emailOrPhone: "+",
			typee:        2,
			expectMsg:    "Телефон принят",
		},
		{ // странный символ
			name:         "Strange email symbol 1",
			emailOrPhone: "0",
			typee:        1,
			expectMsg:    "Почты получателя не существует",
		},
		{ // странный символ
			name:         "Strange phone symbol 1",
			emailOrPhone: "Ъ",
			typee:        2,
			expectMsg:    "Телефон принят",
		},
		{ // странный символ
			name:         "Strange email symbol 2",
			emailOrPhone: "._+",
			typee:        1,
			expectMsg:    "Почты получателя не существует",
		},
		{ // странный символ
			name:         "Strange phone symbol 2",
			emailOrPhone: "._+",
			typee:        2,
			expectMsg:    "Телефон принят",
		},
	}

	for _, tc := range cases { // запускаем цикл, который бы прогнал тест кейсы
		t.Run(tc.name, func(t *testing.T) { // запуск теста, где учитывается его имя и выполняется функция
			// Создаем мок Redis
			db, mock := redismock.NewClientMock()

			ctx := context.Background()
			logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr})

			// Передаем мок Redis в обработчик
			handlerData := &user.SignupHandler{
				RedisClient: db, // Заменили реальный Redis
				Logger:      logger,
				CodeNum:     1234,
				JWT:         "JWT",
			}

			if tc.typee == 1 {
				handler := handlerData.SignupUserByEmail(logger, ctx) // сам хендлер
				wrappedHandler := httprouterAdapter(handler)          // обёртка, чтобы он возвращал (http.Handler)

				// Отправляем JSON-запрос
				payload := fmt.Sprintf(`{"Email": "%s"}`, tc.emailOrPhone)                                          // тестовый JSON запрос
				req := httptest.NewRequest(http.MethodPost, "/signupUserByEmail", bytes.NewBuffer([]byte(payload))) // тестовый HTTP-запрос
				rr := httptest.NewRecorder()                                                                        // создание объекта для записи ответа

				type Kesh struct {
					Email string `json:"Email"`
					Code  int    `json:"Code"`
				}

				// Сохранение кода в Redis
				keshData, err := json.Marshal(Kesh{Email: tc.emailOrPhone, Code: handlerData.CodeNum})
				if err != nil {
					// logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка сериализации JSON", op))

					return
				}

				// Ожидаем, что в Redis будет сделана запись с этим ключом
				mock.MatchExpectationsInOrder(false)
				mock.ExpectSet(handlerData.JWT, string(keshData), 40*time.Minute).SetVal("OK")

				wrappedHandler.ServeHTTP(rr, req) // вызов хендлера
				// Проверяем тело ответа
				var resp map[string]interface{}                            // карта для распарсенных данных ответа
				require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp)) // получает тело HTTP-ответа

				require.Equal(t, tc.expectMsg, resp["message"].(string)) // сверяет сообщения, заложенные в тестах и полученные сорваком

				require.Equal(t, http.StatusOK, rr.Code) // проверка кода ответа
				// После вызова проверяем код ответа
			} else {
				type Kesh struct {
					Phone_number string `json:"Phone_num"`
					Code         int    `json:"Code"`
				}

				// 1. Поднимаем локальный мок-сервер
				mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Преобразуем структуру kesh в JSON

					// Пишем нужный ответ
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`
						{
						"status": "ok",
						"data": {
							"balance": "333.333333",
							"call_id": 999999999999999,
							"created": "2023-02-09T12:50:55.621Z",
							"phone": "+77999999999",
							"pincode": "9999"
						}
						}
 					 `))
				}))
				defer mockServer.Close()

				// Создаём хендлер, в котором requestURL указывает на локальный сервер
				handlerData := &user.SignupHandler{
					RedisClient: db,
					CodeNum:     1234,
					JWT:         "JWT",
					RequestURL:  mockServer.URL,
				}

				// Отправляем JSON-запрос
				payload := fmt.Sprintf(`{"Phone_num": "%s"}`, tc.emailOrPhone)                                      // тестовый JSON запрос
				req := httptest.NewRequest(http.MethodPost, "/signupUserByPhone", bytes.NewBuffer([]byte(payload))) // тестовый HTTP-запрос
				rr := httptest.NewRecorder()                                                                        // создание объекта для записи ответа

				// Сохранение кода в Redis
				keshData, err := json.Marshal(Kesh{Phone_number: tc.emailOrPhone, Code: handlerData.CodeNum})
				if err != nil {
					// logger.Err(err).Msg(fmt.Sprintf("error in %s; ошибка сериализации JSON", op))

					return
				}

				// Ожидаем, что в Redis будет сделана запись с этим ключом
				mock.MatchExpectationsInOrder(false)
				mock.ExpectSet(handlerData.JWT, string(keshData), 40*time.Minute).SetVal("OK")

				handler := handlerData.SignupUserByPhone(logger, ctx)
				handler(rr, req, nil)

				// Разбираем JSON-ответ
				var resp map[string]interface{}
				err = json.Unmarshal(rr.Body.Bytes(), &resp) // разбираем JSON
				if err != nil {
					fmt.Println("Ошибка парсинга JSON:", err)
					return
				}

				require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp)) // получает тело HTTP-ответа

				require.Equal(t, tc.expectMsg, resp["message"].(string)) // сверяет сообщения, заложенные в тестах и полученные сорваком

				require.Equal(t, http.StatusOK, rr.Code) // проверка кода ответа
				// После вызова проверяем код ответа
			}
		})
	}
}

/*
	fmt.Println("Info: ", resp)
	fmt.Println("Data: ", resp["data"])
	fmt.Println("Message: ", resp["message"])
*/

func httprouterAdapter(h httprouter.Handle) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h(w, r, httprouter.Params{})
	})
}
