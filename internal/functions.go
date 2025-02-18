package internal

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"myproject/internal/model"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// Генерация имени файла на основе временной метки
func GenerateFileName(extension, user_id, index string) string {
	timestamp := time.Now().Format("010203084503") // ГГГГММДДччммсс

	return fmt.Sprintf("image_%s_%s_%s.%s", timestamp, user_id, index, extension)
}

func UploadImage(rw http.ResponseWriter, imageBase64, directory, user_id, index string, logger zerolog.Logger) (error, bool, string) {
	op := "internal.function.UploadImage"

	// Проверяем, содержит ли строка базовые метаданные
	if strings.HasPrefix(imageBase64, "data:image/png;base64,") {
		// Извлекаем данные после запятой
		commaIndex := strings.Index(imageBase64, ",")
		if commaIndex == -1 {
			http.Error(rw, "Invalid base64 data", http.StatusBadRequest)
			return fmt.Errorf("invalid base64 data"), false, ""
		}
		imageBase64 = imageBase64[commaIndex+1:]
	}

	// Декодируем данные base64
	data, err := base64.StdEncoding.DecodeString(imageBase64)
	if err != nil {
		logger.Err(err).Msg(fmt.Sprintf("error in ", op, "; ошибка с декодингом данных base64"))
	}

	// Генерируем уникальное имя файла
	fileName := GenerateFileName("png", user_id, index)

	// Полный путь до файла
	filePath := filepath.Join(directory, fileName)

	// Проверяем и создаём директорию, если она не существует
	if _, err := os.Stat(directory); os.IsNotExist(err) {
		err = os.MkdirAll(directory, 0755) // Создаем директорию с правами 0755
		if err != nil {
			http.Error(rw, fmt.Sprintf("Error creating directory %s: %v", directory, err), http.StatusInternalServerError)
			return err, false, ""
		}
	}

	// Создаем файл на сервере для сохранения декодированного файла
	dst, err := os.Create(filePath) // Используем безопасное имя файла
	if err != nil {
		http.Error(rw, fmt.Sprintf("Error creating file %s: %v", filePath, err), http.StatusInternalServerError)
		return err, false, ""
	}
	defer dst.Close()

	// Записываем данные в файл
	if _, err := dst.Write(data); err != nil {
		http.Error(rw, fmt.Sprintf("Error writing to file %s: %v", fileName, err), http.StatusInternalServerError)
		return err, false, ""
	}

	return nil, true, filePath
}

func UploadImagesMass(rw http.ResponseWriter, images []string, pwd string, user_id string, logger zerolog.Logger) (error, bool, []string) {
	var Pwd_path []string
	for image := range images {
		_, _, element := UploadImage(rw, images[image], pwd, user_id, strconv.Itoa(image), logger)

		Pwd_path = append(Pwd_path, element)
	}

	return nil, true, Pwd_path
}

// Функция для массовой загрузки множества видеофайлов
func UploadVideosMass(rw http.ResponseWriter, videos []string, pwd, user_id string, logger zerolog.Logger) (_ error, _ bool, file_path []string) {
	for i := range videos {
		err, success, file := UploadVideo(rw, videos[i], pwd, user_id, strconv.Itoa(i), logger)
		if err != nil || !success {
			return err, false, nil
		}
		file_path = append(file_path, file)
	}
	return nil, true, file_path
}

// // Функция для загрузки одного видеофайла
func UploadVideo(rw http.ResponseWriter, videoBase64, directory, user_id, index string, logger zerolog.Logger) (error, bool, string) {
	// Проверяем, содержит ли строка базовые метаданные для MP4
	if strings.HasPrefix(videoBase64, "data:video/mp4;base64,") {
		// Извлекаем данные после запятой
		commaIndex := strings.Index(videoBase64, ",")
		if commaIndex != -1 {
			videoBase64 = videoBase64[commaIndex+1:]
		}
	}

	// Декодируем данные base64
	data, err := base64.StdEncoding.DecodeString(videoBase64)
	if err != nil {
		http.Error(rw, fmt.Sprintf("Error decoding base64: %v", err), http.StatusInternalServerError)
		return err, false, ""
	}

	// Генерируем уникальное имя файла
	fileName := GenerateFileName("mp4", user_id, index)

	// Полный путь до файла
	filePath := filepath.Join(directory, fileName)

	// Проверяем и создаём директорию, если она не существует
	if _, err := os.Stat(directory); os.IsNotExist(err) {
		err = os.MkdirAll(directory, 0755)
		if err != nil {
			http.Error(rw, fmt.Sprintf("Error creating directory %s: %v", directory, err), http.StatusInternalServerError)
			return err, false, ""
		}
	}

	// Создаем файл на сервере для сохранения декодированного видео
	dst, err := os.Create(filePath)
	if err != nil {
		http.Error(rw, fmt.Sprintf("Error creating file %s: %v", filePath, err), http.StatusInternalServerError)
		return err, false, ""
	}
	defer dst.Close()

	// Записываем данные в файл
	if _, err := dst.Write(data); err != nil {
		http.Error(rw, fmt.Sprintf("Error writing to file %s: %v", fileName, err), http.StatusInternalServerError)
		return err, false, ""
	}

	return nil, true, filePath
}

func DeleteImage(rw http.ResponseWriter, pwd, imageName string) (error, bool) {
	// Формируем полный путь к файлу
	filePath := pwd + imageName

	// Удаляем файл
	err := os.Remove(filePath)
	if err != nil {

	}

	return nil, true
}

func UploadAvatar(rw http.ResponseWriter, imageBase64, directory, user_id, index string) (error, bool, string) {
	// Проверяем, содержит ли строка базовые метаданные
	if strings.HasPrefix(imageBase64, "data:image/png;base64,") {
		// Извлекаем данные после запятой
		commaIndex := strings.Index(imageBase64, ",")
		if commaIndex == -1 {
			http.Error(rw, "Invalid base64 data", http.StatusBadRequest)
			return fmt.Errorf("invalid base64 data"), false, ""
		}
		imageBase64 = imageBase64[commaIndex+1:]
	}

	// Декодируем данные base64
	data, err := base64.StdEncoding.DecodeString(imageBase64)
	if err != nil {
		http.Error(rw, fmt.Sprintf("Error decoding base64: %v", err), http.StatusInternalServerError)
		return err, false, ""
	}

	// Генерируем уникальное имя файла
	fileName := GenerateFileName("png", user_id, index)

	// Полный путь до файла
	filePath := filepath.Join(directory, fileName)

	// Проверяем и создаём директорию, если она не существует
	if _, err := os.Stat(directory); os.IsNotExist(err) {
		err = os.MkdirAll(directory, 0755) // Создаем директорию с правами 0755
		if err != nil {
			http.Error(rw, fmt.Sprintf("Error creating directory %s: %v", directory, err), http.StatusInternalServerError)
			return err, false, ""
		}
	}

	dst, err := os.Create(filePath) // Используем безопасное имя файла
	if err != nil {
		http.Error(rw, fmt.Sprintf("Error creating file %s: %v", filePath, err), http.StatusInternalServerError)
		return err, false, ""
	}
	defer dst.Close()

	// Записываем данные в файл
	if _, err := dst.Write(data); err != nil {
		http.Error(rw, fmt.Sprintf("Error writing to file %s: %v", fileName, err), http.StatusInternalServerError)
		return err, false, ""
	}

	return nil, true, filePath
}

func ValidatePassword(rw http.ResponseWriter, password string) bool {
	if len(password) < 8 {
		response := model.Response{
			Status:  "falat",
			Message: "длина пароля должна быть не менее 8 символов",
		}

		json.NewEncoder(rw).Encode(response)

		return false
	}

	// Проверка наличия хотя бы одной цифры
	if matched, _ := regexp.MatchString(`[0-9]`, password); !matched {
		response := model.Response{
			Status:  "falat",
			Message: "пароль должен содержать хотя бы одну цифру",
		}

		json.NewEncoder(rw).Encode(response)

		return false
	}

	// Проверка наличия хотя бы одной строчной буквы
	if matched, _ := regexp.MatchString(`[a-z]`, password); !matched {
		response := model.Response{
			Status:  "falat",
			Message: "пароль должен содержать хотя бы одну строчную букву",
		}

		json.NewEncoder(rw).Encode(response)

		return false
	}

	// Проверка наличия хотя бы одной заглавной буквы
	if matched, _ := regexp.MatchString(`[A-Z]`, password); !matched {
		response := model.Response{
			Status:  "falat",
			Message: "пароль должен содержать хотя бы одну заглавную букву",
		}

		json.NewEncoder(rw).Encode(response)

		return false
	}

	// Проверка наличия хотя бы одного специального символа
	// if matched, _ := regexp.MatchString(`*[!@#~$%^&*()_+{}":;'?/>.<,]`, password); !matched {
	// 	response := Response{
	// 		Status:  "falat",
	// 		Message: "пароль должен содержать хотя бы один специальный символ",
	// 	}

	// 	json.NewEncoder(rw).Encode(response)

	// 	return false
	// }

	return true
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
