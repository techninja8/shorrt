package main

import (
	"context"
	"database/sql"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/go-redis/redis/v8"
	_ "github.com/mattn/go-sqlite3"
	"github.com/skip2/go-qrcode"
)

type URL struct {
	ID             int       `json:"id"`
	Original       string    `json:"original" validate:"required,url"`
	Short          string    `json:"short"`
	QRCode         string    `json:"qr_code"`
	CustomShort    string    `json:"custom_short"`
	ExpirationDate time.Time `json:"expiration_date"`
	AccessCount    int       `json:"access_count"`
}

var (
	db       *sql.DB
	rdb      *redis.Client
	ctx      = context.Background()
	validate *validator.Validate
)

const (
	shortLinkLength = 6
	charset         = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

var seededRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

func stringWithCharset(length int, charset string) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

func generateShortLink() string {
	return stringWithCharset(shortLinkLength, charset)
}

func main() {
	var err error
	db, err = sql.Open("sqlite3", "./urlshortener.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	rdb = redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_ADDR"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS urls (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		original TEXT NOT NULL,
		short TEXT NOT NULL UNIQUE,
		qr_code TEXT NOT NULL,
		custom_short TEXT UNIQUE,
		expiration_date DATETIME,
		access_count INTEGER DEFAULT 0
	);`)
	if err != nil {
		log.Fatal(err)
	}

	validate = validator.New()

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	router.POST("/shorten", shortenURL)
	router.GET("/:short", redirectURL)
	router.GET("/urls", getURLs)

	// Serve the static HTML file
	router.StaticFile("/", "./index.html")

	// Start the server
	log.Println("Starting server on :8080")
	err = router.Run(":8080")
	if err != nil {
		log.Fatal("Server failed to start: ", err)
	}
}

func shortenURL(c *gin.Context) {
	var requestURL URL
	if err := c.BindJSON(&requestURL); err != nil {
		log.Println("Error binding JSON: ", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := validate.Struct(requestURL); err != nil {
		log.Println("Validation error: ", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if requestURL.CustomShort != "" {
		requestURL.Short = requestURL.CustomShort
	} else {
		requestURL.Short = generateShortLink()
	}

	// Generate QR code
	qrCodeFile := "qrcodes/" + requestURL.Short + ".png"
	err := qrcode.WriteFile(requestURL.Short, qrcode.Medium, 256, qrCodeFile)
	if err != nil {
		log.Println("Error generating QR code: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	requestURL.QRCode = qrCodeFile

	result, err := db.Exec("INSERT INTO urls (original, short, qr_code, custom_short, expiration_date) VALUES (?, ?, ?, ?, ?)",
		requestURL.Original, requestURL.Short, requestURL.QRCode, requestURL.CustomShort, requestURL.ExpirationDate)
	if err != nil {
		log.Println("Error inserting into database: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	id, err := result.LastInsertId()
	if err != nil {
		log.Println("Error getting last insert ID: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	requestURL.ID = int(id)

	// Cache the URL
	err = rdb.Set(ctx, requestURL.Short, requestURL.Original, 0).Err()
	if err != nil {
		log.Println("Error caching URL: ", err)
	}

	c.JSON(http.StatusOK, requestURL)
}

func redirectURL(c *gin.Context) {
	short := c.Param("short")

	// Check cache
	original, err := rdb.Get(ctx, short).Result()
	if err == redis.Nil {
		// Not in cache, query database
		var expirationDate sql.NullTime
		var accessCount int
		err = db.QueryRow("SELECT original, expiration_date, access_count FROM urls WHERE short = ? OR custom_short = ?", short, short).Scan(&original, &expirationDate, &accessCount)
		if err != nil {
			if err == sql.ErrNoRows {
				log.Println("No rows found for short link: ", short)
				c.JSON(http.StatusNotFound, gin.H{"error": "URL not found"})
				return
			}
			log.Println("Error querying database: ", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}

		// Check if the link has expired
		if expirationDate.Valid && expirationDate.Time.Before(time.Now()) {
			log.Println("Link has expired: ", short)
			c.JSON(http.StatusGone, gin.H{"error": "URL has expired"})
			return
		}

		// Increment access count
		accessCount++
		_, err = db.Exec("UPDATE urls SET access_count = ? WHERE short = ? OR custom_short = ?", accessCount, short, short)
		if err != nil {
			log.Println("Error updating access count: ", err)
		}

		// Cache the result
		err = rdb.Set(ctx, short, original, 0).Err()
		if err != nil {
			log.Println("Error caching URL: ", err)
		}
	} else if err != nil {
		log.Println("Error querying cache: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	} else {
		// Increment access count
		var accessCount int
		err = db.QueryRow("SELECT access_count FROM urls WHERE short = ? OR custom_short = ?", short, short).Scan(&accessCount)
		if err != nil {
			log.Println("Error querying database: ", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}
		accessCount++
		_, err = db.Exec("UPDATE urls SET access_count = ? WHERE short = ? OR custom_short = ?", accessCount, short, short)
		if err != nil {
			log.Println("Error updating access count: ", err)
		}
	}

	c.Redirect(http.StatusMovedPermanently, original)
}

func getURLs(c *gin.Context) {
	rows, err := db.Query("SELECT id, original, short, qr_code, custom_short, expiration_date, access_count FROM urls")
	if err != nil {
		log.Println("Error querying database: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	defer rows.Close()

	var urls []URL
	for rows.Next() {
		var url URL
		var expirationDate sql.NullTime
		if err := rows.Scan(&url.ID, &url.Original, &url.Short, &url.QRCode, &url.CustomShort, &expirationDate, &url.AccessCount); err != nil {
			log.Println("Error scanning row: ", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}
		if expirationDate.Valid {
			url.ExpirationDate = expirationDate.Time
		}
		urls = append(urls, url)
	}

	if err := rows.Err(); err != nil {
		log.Println("Error iterating rows: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, urls)
}
