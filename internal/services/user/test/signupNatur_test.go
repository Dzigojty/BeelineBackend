package test

// import (
// 	"context"
// 	"fmt"
// 	"log"
// 	"myproject/internal/database"
// 	"myproject/internal/model"
// 	"testing"
// 	"time"

// 	"encoding/json"
// )

// // Структура тестов
// type signupNaturCase struct {
// 	name         string
// 	email        string
// 	phone        string
// 	typee        int
// 	passwordHash string

// 	userName   string
// 	surname    string
// 	patronymic string

// 	photo   string
// 	message string
// }

// var signupNaturCases = []signupNaturCase{
// 	{"Valid data1", "test.octa.one@gagarin.com", "", 1, "23202004aA/", "Виталий", "OOO.MyCompany", "Россия, РСО-Алания, г.Владикавказ, ул. Весенняя 7д", phocoCase, "The user has been successfully registered"},
// 	{"Valid data2", "", "+7(928)074-32-44", 2, "23202004aA/", "Виталий", "Витальев", "Виталиевич", phocoCase, "The user has been successfully registered"},

// 	{"Invalid data1", "", "", 1, "23202004aA/", "Виталий", "OOO.MyCompany", "Россия, РСО-Алания, г.Владикавказ, ул. Весенняя 7д", phocoCase, "Пустая почта"},
// 	{"Invalid data2", "", "", 2, "23202004aA/", "Виталий", "OOO.MyCompany", "Россия, РСО-Алания, г.Владикавказ, ул. Весенняя 7д", phocoCase, "Пустой телефон"},

// 	{"Invalid password1", "test.octa.one@gagarin.com", "", 1, "x", "Виталий", "OOO.MyCompany", "Россия, РСО-Алания, г.Владикавказ, ул. Весенняя 7д", phocoCase, "длина пароля должна быть не менее 8 символов"},
// 	{"Invalid password2", "", "+7(928)074-32-44", 2, "x", "Виталий", "OOO.MyCompany", "Россия, РСО-Алания, г.Владикавказ, ул. Весенняя 7д", phocoCase, "длина пароля должна быть не менее 8 символов"},

// 	{"Invalid indNum1", "test.octa.one@gagarin.com", "", 1, "23202004aA/", "", "OOO.MyCompany", "Россия, РСО-Алания, г.Владикавказ, ул. Весенняя 7д", phocoCase, "The user has been successfully registered"},
// 	{"Invalid indNum2", "", "+7(928)074-32-44", 2, "23202004aA/", "", "OOO.MyCompany", "Россия, РСО-Алания, г.Владикавказ, ул. Весенняя 7д", phocoCase, "The user has been successfully registered"},

// 	{"Invalid nameOfCompany1", "test.octa.one@gagarin.com", "", 1, "23202004aA/", "Виталий", "", "Россия, РСО-Алания, г.Владикавказ, ул. Весенняя 7д", phocoCase, "The user has been successfully registered"},
// 	{"Invalid nameOfCompany2", "", "+7(928)074-32-44", 2, "23202004aA/", "Виталий", "", "Россия, РСО-Алания, г.Владикавказ, ул. Весенняя 7д", phocoCase, "The user has been successfully registered"},

// 	{"Invalid addressName1", "test.octa.one@gagarin.com", "", 1, "23202004aA/", "Виталий", "OOO.MyCompany", "", phocoCase, "The user has been successfully registered"},
// 	{"Invalid addressName2", "", "+7(928)074-32-44", 2, "23202004aA/", "Виталий", "OOO.MyCompany", "", phocoCase, "The user has been successfully registered"},

// 	{"Invalid Redis data1", "test.octa.one@gagarin.com", "", 1, "23202004aA/", "Виталий", "OOO.MyCompan", "Россия, РСО-Алания, г.Владикавказ, ул. Весенняя 7д", phocoCase, "The user has been successfully registered"},
// 	{"Invalid Redis data2", "", "+7(928)074-32-44", 2, "23202004aA/", "Виталий", "OOO.MyCompan", "Россия, РСО-Алания, г.Владикавказ, ул. Весенняя 7д", phocoCase, "The user has been successfully registered"},
// }

// func TestSignupNaturHandler(t *testing.T) {
// 	ctx := context.Background()

// 	for _, tc := range signupNaturCases {
// 		t.Run(tc.name, func(t *testing.T) {
// 			handler, mock, _ := setupRedisMok()

// 			handlerPlayload, _ := json.Marshal(map[string]interface{}{
// 				"password_hash": tc.passwordHash,
// 				"Name":          tc.userName,
// 				"Surname":       tc.surname,
// 				"Patronymic":    tc.patronymic,
// 				"data":          phocoCase,
// 			})

// 			if tc.typee == 1 {
// 				keshData, _ := json.Marshal(model.Email_kesh{Email: tc.email, Code: 0})

// 				// Ожидаем, что в Redis будет сделана запись с этим ключом
// 				mock.MatchExpectationsInOrder(false)
// 				// Сначала записываем данные в Redis (чтобы они там были)
// 				mock.ExpectSet(handler.JWT, string(keshData), 40*time.Minute).SetVal("OK")

// 				// Затем эмулируем их чтение
// 				mock.ExpectGet(handler.JWT).SetVal(string(keshData))

// 				// Инициализация подключения к базе данных
// 				dbpool, err := database.InitMockDBConn(ctx)
// 				if err != nil {
// 					log.Fatalf("%v failed to init DB connection", err)
// 				}
// 				defer dbpool.Close()

// 				_, err = dbpool.Exec(ctx, `DELETE FROM finance.wallets;`)
// 				if err != nil {
// 					log.Fatal(fmt.Sprintf("Error in SignupLegal test; Ошибка при очистке БД перед тестом: %s", err))

// 					return
// 				}

// 				_, err = dbpool.Exec(ctx, `DELETE FROM users.individual_user;`)
// 				if err != nil {
// 					log.Fatal(fmt.Sprintf("Error in SignupLegal test; Ошибка при очистке БД перед тестом: %s", err))

// 					return
// 				}

// 				_, err = dbpool.Exec(ctx, `DELETE FROM users.users;`)
// 				if err != nil {
// 					log.Fatal(fmt.Sprintf("Error in SignupLegal test; Ошибка при очистке БД перед тестом: %s", err))

// 					return
// 				}

// 				// Теперь создаём сам хендлер, передавая мок
// 				handlerFunc := handler.SignupNaturEmail(ctx, dbpool)
// 				testRequest(t, httprouterAdapterEnterCod(handlerFunc), string(handlerPlayload), tc.message, "/signupNaturEmail")

// 				_, err = dbpool.Exec(ctx, `DELETE FROM finance.wallets;`)
// 				if err != nil {
// 					log.Fatal(fmt.Sprintf("Error in SignupLegal test; Ошибка при очистке БД перед тестом: %s", err))

// 					return
// 				}

// 				_, err = dbpool.Exec(ctx, `DELETE FROM users.individual_user;`)
// 				if err != nil {
// 					log.Fatal(fmt.Sprintf("Error in SignupLegal test; Ошибка при очистке БД перед тестом: %s", err))

// 					return
// 				}

// 				_, err = dbpool.Exec(ctx, `DELETE FROM users.users;`)
// 				if err != nil {
// 					log.Fatal(fmt.Sprintf("Error in SignupLegal test; Ошибка при очистке БД перед тестом: %s", err))

// 					return
// 				}

// 				return
// 			}

// 			// if tc.typee == 2 {
// 			// 	keshData, _ := json.Marshal(model.Phone_kesh{Phone: tc.phone, Code: 0})

// 			// 	// Ожидаем, что в Redis будет сделана запись с этим ключом
// 			// 	mock.MatchExpectationsInOrder(false)
// 			// 	// Сначала записываем данные в Redis (чтобы они там были)
// 			// 	mock.ExpectSet(handler.JWT, string(keshData), 40*time.Minute).SetVal("OK")

// 			// 	// Затем эмулируем их чтение
// 			// 	mock.ExpectGet(handler.JWT).SetVal(string(keshData))

// 			// 	// Инициализация подключения к базе данных
// 			// 	dbpool, err := database.InitMockDBConn(ctx)
// 			// 	if err != nil {
// 			// 		log.Fatalf("%v failed to init DB connection", err)
// 			// 	}
// 			// 	defer dbpool.Close()

// 			// 	_, err = dbpool.Exec(ctx, `DELETE FROM finance.wallets;`)
// 			// 	if err != nil {
// 			// 		log.Fatal(fmt.Sprintf("Error in SignupLegal test; Ошибка при очистке БД перед тестом: %s", err))

// 			// 		return
// 			// 	}

// 			// 	_, err = dbpool.Exec(ctx, `DELETE FROM users.individual_user;`)
// 			// 	if err != nil {
// 			// 		log.Fatal(fmt.Sprintf("Error in SignupLegal test; Ошибка при очистке БД перед тестом: %s", err))

// 			// 		return
// 			// 	}

// 			// 	_, err = dbpool.Exec(ctx, `DELETE FROM users.users;`)
// 			// 	if err != nil {
// 			// 		log.Fatal(fmt.Sprintf("Error in SignupLegal test; Ошибка при очистке БД перед тестом: %s", err))

// 			// 		return
// 			// 	}

// 			// 	// Теперь создаём сам хендлер, передавая мок
// 			// 	handlerFunc := handler.SignupNaturPhone(ctx, dbpool)
// 			// 	testRequest(t, httprouterAdapterEnterCod(handlerFunc), string(handlerPlayload), tc.message, "/signupNaturPhone")

// 			// 	_, err = dbpool.Exec(ctx, `DELETE FROM finance.wallets;`)
// 			// 	if err != nil {
// 			// 		log.Fatal(fmt.Sprintf("Error in SignupLegal test; Ошибка при очистке БД перед тестом: %s", err))

// 			// 		return
// 			// 	}

// 			// 	_, err = dbpool.Exec(ctx, `DELETE FROM users.individual_user;`)
// 			// 	if err != nil {
// 			// 		log.Fatal(fmt.Sprintf("Error in SignupLegal test; Ошибка при очистке БД перед тестом: %s", err))

// 			// 		return
// 			// 	}

// 			// 	_, err = dbpool.Exec(ctx, `DELETE FROM users.users;`)
// 			// 	if err != nil {
// 			// 		log.Fatal(fmt.Sprintf("Error in SignupLegal test; Ошибка при очистке БД перед тестом: %s", err))

// 			// 		return
// 			// 	}

// 			// 	return
// 			// }
// 		})
// 	}
// }
