package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v4"
	"github.com/spf13/viper"
)

type RequestData struct {
	Module string                 `json:"module"`
	Type   string                 `json:"type"`
	Event  string                 `json:"event"`
	Name   string                 `json:"name"`
	Data   map[string]interface{} `json:"data"`
}

type AnalyticsData struct {
	Headers map[string]interface{} `json:"headers"`
	Body    RequestData            `json:"body"`
}

func SetMode(mode string) {
	switch mode {
	case "release":
		gin.SetMode(gin.ReleaseMode)
	case "debug":
		gin.SetMode(gin.DebugMode)
	case "test":
		gin.SetMode(gin.TestMode)
	default:
		panic("mode unavailable. (debug, release, test)")
	}
}

func main() {
	// Read path from config file
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		log.Println("Fatal error config file:", err)
		os.Exit(1)
	}

	dbConfig := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		viper.GetString("db.HOST"), viper.GetInt("db.PORT"), viper.GetString("db.USER"),
		viper.GetString("db.PASSWORD"), viper.GetString("db.DBNAME"), viper.GetString("db.SSLMODE"),
	)

	// подключаемся к базе данных
	conn, err := pgx.Connect(context.Background(), dbConfig)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close(context.Background())

	// создаём таблицу, если её нет
	createTable := `CREATE TABLE IF NOT EXISTS analytics (
        time TIMESTAMP,
        user_id VARCHAR(255),
        data jsonb
    )`
	_, err = conn.Exec(context.Background(), createTable)
	if err != nil {
		log.Fatal(err)
	}

	// создаём буферизированный канал для обработанных запросов
	processedRequests := make(chan *http.Request, 100)

	// запускаем воркер-пулл для обработки запросов
	for i := 0; i < 10; i++ {
		go requestWorker(i, processedRequests, conn)
	}

	SetMode(viper.GetString("gin.mode"))
	r := gin.Default()

	r.POST("/analytics", func(c *gin.Context) {
		userID := c.Request.Header.Get("X-Tantum-Authorization")
		if userID == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "No user ID in the request header"})
			return
		}

		userAgent := c.Request.Header.Get("X-Tantum-UserAgent")
		if userAgent == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "No user Agent in the request header"})
			return
		}

		// считываем тело запроса
		var requestData RequestData
		bodyBytes, err := ioutil.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// восстанавливаем тело запроса
		c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))

		// декодируем JSON в requestData
		err = json.Unmarshal(bodyBytes, &requestData)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// отправляем запрос в канал для обработки
		copyContext := c.Copy()
		copyRequest := c.Request.WithContext(copyContext)

		processedRequests <- copyRequest

		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	port := viper.GetString("server.port")
	fmt.Printf("Server is running on port %s...\n", port)
	err = r.Run(":" + port)
	if err != nil {
		fmt.Println(err)
	}
}

type Headers struct {
	UserId    string `json:"X-Tantum-Authorization"`
	UserAgent string `json:"X-Tantum-UserAgent"`
}

func requestWorker(workerID int, processedRequests <-chan *http.Request, conn *pgx.Conn) {
	for r := range processedRequests {
		// обрабатываем запрос
		time.Sleep(5 * time.Second)

		// сохраняем данные из тела запроса

		reqBody, _ := ioutil.ReadAll(r.Body)
		var requestData RequestData
		json.Unmarshal(reqBody, &requestData)

		newData, err := json.Marshal(requestData)
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println(string(newData))
		}

		var analyticsData AnalyticsData
		analyticsData.Body = requestData

		var headers Headers
		headers.UserAgent = r.Header.Get("X-Tantum-UserAgent")
		headers.UserId = r.Header.Get("X-Tantum-Authorization")

		reqHeaders, _ := json.Marshal(&headers)
		json.Unmarshal(reqHeaders, &analyticsData.Headers)

		// сохраняем данные в базу данных в другой горутине
		go func(data AnalyticsData, userID string) {
			dataJSON, _ := json.Marshal(data)
			insertQuery := fmt.Sprintf("INSERT INTO analytics (time, user_id, data) VALUES ('%v', '%v', '%v')", time.Now().Format("2006-01-02 15:04:05"), userID, string(dataJSON))
			_, err = conn.Exec(context.Background(), insertQuery)
			if err != nil {
				log.Println(err)
			}
		}(analyticsData, r.Header.Get("X-Tantum-Authorization"))
	}
}
