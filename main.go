package main

import (
	"context"
	"fmt"
	"log"
	"myproject/internal/app"
	"myproject/internal/database"
	"net/http"
	"os"
	"os/exec"

	"github.com/go-redis/redis/v8"
	"github.com/julienschmidt/httprouter"
	"github.com/natefinch/lumberjack"
	"github.com/rs/zerolog"
)

const apiUrl = "https://api.t-bank.com/v1/nominal-accounts" // Инициализация клиента Redis
func NewRedisClient() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     "127.0.0.1:6379", // Проверьте, что адрес и порт верные
		Password: "",               // Если Redis настроен без пароля
		DB:       0,                // Используйте базу данных 0
	})
}

var redisClient = NewRedisClient() // Глобальный клиент Redis

func main() {
	ctx := context.Background()
	// Инициализация подключения к базе данных
	dbpool, err := database.InitDBConn(ctx)
	if err != nil {
		log.Fatalf("%v failed to init DB connection", err)
	}
	defer dbpool.Close()

	logFile := &lumberjack.Logger{
		Filename:   "/root/home/beeline/beeline.log", //Имя лог-файла
		MaxSize:    10,                               // Максимальный размер файла в МБ
		MaxBackups: 5,                                // Максимальное количество копий
		MaxAge:     30,                               // Хранить логи 30 дней
		Compress:   true,
	}

	logger := zerolog.New(logFile).With().Timestamp().Logger()
	logger.Info().Msg("Start server")

	// Инициализация приложения
	a := app.NewApp(ctx, dbpool)
	r := httprouter.New()

	// Определяем маршруты для приложения
	a.Routes(r, ctx, dbpool, redisClient, logger)

	// Применяем CORS middleware ко всем маршрутам
	handlerWithCORS := corsMiddleware(r)

	// Настройка сервера
	srv := &http.Server{Addr: "45.134.12.241:8070", // 185.112.83.36.36
		Handler: handlerWithCORS, // Используем обработчик с поддержкой CORS
	}

	// этап проверки тестов и их вывод
	fmt.Println("Запуск тестов...")

	cmd := exec.Command("go", "test", "./internal/services/user/test/...", "-v")
	cmd.Stdout = os.Stdout // Направляем вывод в стандартный поток
	cmd.Stderr = os.Stderr // Направляем ошибки в стандартный поток

	err = cmd.Run()
	if err != nil {
		fmt.Printf("Тесты завершились с ошибкой: %v\n", err)
		os.Exit(1) // Завершаем с ненулевым кодом при ошибке
	}

	fmt.Println("Все тесты пройдены успешно.")
	// конец этапа

	// Запуск сервера
	fmt.Println("Сервер запущен на http://45.134.12.241:8070")
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("Не удалось запустить сервер: %s\n", err)
	}
}

// Middleware для добавления CORS-заголовков
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		// Устанавливаем CORS-заголовки
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		// Если это preflight-запрос OPTIONS, возвращаем 200 OK
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		// Передаем управление следующему обработчику
		next.ServeHTTP(w, r)
	})
}
