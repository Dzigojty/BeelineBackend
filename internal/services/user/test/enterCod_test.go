package test

import (
	"bytes"
	"context"
	"encoding/json"
	"myproject/internal/services/user"
	"net/http"
	"net/http/httptest"
	"os"

	"testing"

	"github.com/go-redis/redismock/v8"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

// Преобразует `httprouter.Handle` в `http.HandlerFunc`
func httprouterAdapterEnterCod(h httprouter.Handle) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h(w, r, httprouter.Params{}) // передаём пустые параметры
	}
}

// Структура тестов
type testCase struct {
	name          string      // имя теста, в котором содержится его кратакя характеристика
	emailOrPhone  string      // почта или телефон, на который пришёл код
	expectCode    interface{} // код из ответа
	request_token string      // токен для получения информации(кода) из кеша
	typee         int         // это нужно для тестов (1 - регистрация через имейл, 2 - через номер телефона)
	expectMsg     string      // JSON ответ
	// симулируя ошибочное поведение зависимостей. Оно используется для проверки, как хендлер обрабатывает внутренние ошибки.
}

var cases = []testCase{
	{"Valid code from email", "test.octa.one@gmail.com", 1234, "true_jwt", 1, "Код принят"},
	{"Valid code from phone", "+7(928)074-32-44", 1234, "true_jwt", 2, "Код принят"},
	{"Valid code and invalid email", "test.octa.oneee@gmail.com", 0000, "true_jwt", 1, "Неверный код"},
	{"Valid code and invalid phone", "+7(928)074-32-44я", 0000, "true_jwt", 2, "Неверный код"},
	{"Invalid code from email", "test.octa.one@gmail.com", "папа", "true_jwt", 1, "Неверный формат кода"},
	{"Invalid code from phone", "+7(928)074-32-44", "мама", "true_jwt", 2, "Неверный формат кода"},
	{"Unknown email", "test.octa.two@gmail.com", 1234, "false_jwt", 1, "Неверный формат кода"},
	{"Unknown phone", "+7(928)074-32-45", 1234, "false_jwt", 2, "Неверный формат кода"},
}

// Вспомогательная функция: Создаёт Redis мок и возвращает его вместе с обработчиком
func setupRedisMok() (*user.SignupHandler, redismock.ClientMock, zerolog.Logger) {
	// Создаем мок Redis
	db, mock := redismock.NewClientMock()
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr})
	// Передаем мок Redis в обработчик
	handlerData := &user.SignupHandler{
		RedisClient: db, // Заменили реальный Redis
		Logger:      logger,
		CodeNum:     1234,
		JWT:         "true_jwt",
	}
	return handlerData, mock, logger
}

// Вспомогательная функция: Отправляет тестовый HTTP-запрос и проверяет ответ
func testRequest(t *testing.T, handler http.HandlerFunc, payload string, expectMsg string, path string) {
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBuffer([]byte(payload)))
	rr := httptest.NewRecorder()
	handler(rr, req)

	// Разбираем JSON-ответ
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))

	// Проверяем сообщение и статус ответа
	require.Equal(t, expectMsg, resp["message"].(string))
	require.Equal(t, http.StatusOK, rr.Code)
}

func TestEnterCodeHandler(t *testing.T) {
	ctx := context.Background()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler, mock, _ := setupRedisMok()

			if tc.typee == 1 {
				handlerPlayload, _ := json.Marshal(map[string]interface{}{
					"Email": "",
					"Code":  tc.expectCode,
				})

				// Данные для Redis
				keshData, _ := json.Marshal(map[string]interface{}{
					"Email": tc.emailOrPhone,
					"Code":  handler.CodeNum,
				})

				// тут будет эмитация пердыдущего хендлера
				if tc.emailOrPhone == "test.octa.oneee@gmail.com" {
					// Данные для Redis
					kesh, _ := json.Marshal(map[string]interface{}{
						"Email": "nil",
						"Code":  12345,
					})

					// Настраиваем Redis мок
					mock.MatchExpectationsInOrder(false)
					mock.ExpectGet(handler.JWT).SetVal(string(kesh))

					handlerFunc := handler.EnterCodeFromEmail(ctx)
					testRequest(t, httprouterAdapterEnterCod(handlerFunc), string(handlerPlayload), tc.expectMsg, "/enterCodeFromEmail")

					return
				}

				if tc.emailOrPhone == "test.octa.two@gmail.com" {
					handlerFunc := handler.EnterCodeFromEmail(ctx)
					testRequest(t, httprouterAdapterEnterCod(handlerFunc), string(""), tc.expectMsg, "/enterCodeFromPhone")

					return
				}

				// Настраиваем Redis мок
				mock.MatchExpectationsInOrder(false)
				mock.ExpectGet(handler.JWT).SetVal(string(keshData))

				handlerFunc := handler.EnterCodeFromEmail(ctx)
				testRequest(t, httprouterAdapterEnterCod(handlerFunc), string(handlerPlayload), tc.expectMsg, "/enterCodeFromEmail")
			} else {
				handlerPlayload, _ := json.Marshal(map[string]interface{}{
					"Email": "",
					"Code":  tc.expectCode,
				})

				// Данные для Redis
				keshData, _ := json.Marshal(map[string]interface{}{
					"Phone_num": tc.emailOrPhone,
					"Code":      handler.CodeNum,
				})

				if tc.emailOrPhone == "+7(928)074-32-44я" {
					// Данные для Redis
					kesh, _ := json.Marshal(map[string]interface{}{
						"Phone_num": "nil",
						"Code":      12345,
					})

					// Настраиваем Redis мок
					mock.MatchExpectationsInOrder(false)
					mock.ExpectGet(handler.JWT).SetVal(string(kesh))

					handlerFunc := handler.EnterCodeFromEmail(ctx)
					testRequest(t, httprouterAdapterEnterCod(handlerFunc), string(handlerPlayload), tc.expectMsg, "/enterCodeFromPhone")

					return
				}

				if tc.emailOrPhone == "+7(928)074-32-45" {
					handlerFunc := handler.EnterCodeFromEmail(ctx)
					testRequest(t, httprouterAdapterEnterCod(handlerFunc), string(""), tc.expectMsg, "/enterCodeFromPhone")

					return
				}

				// Настраиваем Redis мок
				mock.MatchExpectationsInOrder(false)
				mock.ExpectGet(handler.JWT).SetVal(string(keshData))

				handlerFunc := handler.EnterCodeFromEmail(ctx)
				testRequest(t, httprouterAdapterEnterCod(handlerFunc), string(handlerPlayload), tc.expectMsg, "/enterCodeFromPhone")
			}
		})
	}
}
