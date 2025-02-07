package jwt

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/golang-jwt/jwt"
)

// MyCustomClaims - структура для хранения утверждений токена
type MyCustomClaims struct {
	Foo string `json:"foo"`
	jwt.StandardClaims
}

var mySigningKey = []byte("superSecretKey") //секретнйы ключь для подписи

func GenerateJWT(name string, user_id, user_role int) (string, error) { //функция, которая возвращает строковый формат JWT
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{ //токен, который выдают пользователю
		"name":      name,
		"user_id":   user_id,
		"user_role": user_role,
		"exp":       time.Now().Add(365 * 24 * time.Hour).Unix(), // 1 год
		// "exp":  time.Now().Add(time.Minute * 30).Unix(), //время жизни токена
	})

	tokenString, err := token.SignedString(mySigningKey) //генерация токена в строковом формате
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// Функция для проверки токена и извлечения полезной нагрузки
func IsAuthorized(tokenString string) (bool, int, int) {
	// Парсинг и верификация токена
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Проверка метода подписи токена
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return mySigningKey, nil
	})

	if err != nil {
		return false, 0, 0 // Ошибка при парсинге или верификации токена
	}

	// Проверка валидности токена
	if !token.Valid {
		return false, 0, 0
	}

	// Извлечение полезной нагрузки (claims) из токена
	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		// Извлекаем user_id из claims
		userIDFloat, idFlag := claims["user_id"].(float64)
		userRoleFloat, roleFlag := claims["user_role"].(float64)
		if idFlag && roleFlag {
			// Приводим float64 к int
			return true, int(userIDFloat), int(userRoleFloat)
		}
		return false, 0, 0
	}
	return false, 0, 0
}

func ReadCookie(name string, r *http.Request) (value string, err error) {
	if name == "" {
		// log.Println("Trying to read an empty cookie name")
		return value, errors.New("you are trying to read empty cookie")
	}
	cookie, err := r.Cookie(name)
	if err != nil {
		// log.Printf("Cookie %s not found: %v\n", name, err)
		return value, err
	}
	str := cookie.Value
	value, _ = url.QueryUnescape(str)
	// log.Printf("Cookie %s found with value: %s\n", name, value)
	return value, err
}
