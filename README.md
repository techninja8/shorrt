# Shorrt: Go-Powered URL Link Shortner

Shorrt is a URL shortening service implemented in Go. It allows users to shorten long URLs into short, unique links, optionally customize the short links, track access counts, and set expiration dates. It also generates QR codes for each shortened URL. This document provides detailed information about the architecture, functionalities, and usage of Shorrt.

---

## Features
1. **URL Shortening**: Shorten long URLs into unique short links.
2. **Custom Short Links**: Optionally specify a custom short link.
3. **Expiration Dates**: Set an expiration date for short links.
4. **Access Count Tracking**: Track the number of times a short link is accessed.
5. **QR Code Generation**: Automatically generate a QR code for each short link.
6. **Web Interface**: Manage short links using a web interface.
7. **Performance**: Uses SQLite for persistent storage and Redis for caching.

---

## Architecture

Shorrt consists of:
- **Gin Framework**: Handles HTTP routing and request processing.
- **SQLite Database**: Stores URLs, custom short links, expiration dates, and access counts.
- **Redis**: Caches short links for better performance.
- **QR Code Generator**: Generates QR codes for each short link.

---

## Endpoints

### 1. Create a Shortened URL
**Endpoint**: `POST /shorten`  
**Description**: Shorten a URL, optionally specify a custom short link, and set an expiration date.

**Request Body**:
```json
{
  "original": "https://www.example.com",
  "custom_short": "example",       // Optional
  "expiration_date": "2025-04-30T23:59:59Z" // Optional
}
```

**Response**:
```json
{
  "id": 1,
  "original": "https://www.example.com",
  "short": "example",
  "qr_code": "qrcodes/example.png",
  "custom_short": "example",
  "expiration_date": "2025-04-30T23:59:59Z",
  "access_count": 0
}
```

**Example (cURL)**:
```bash
curl -X POST http://localhost:8080/shorten -H "Content-Type: application/json" -d '{"original":"https://www.example.com", "custom_short":"example", "expiration_date":"2025-04-30T23:59:59Z"}'
```

---

### 2. Redirect to Original URL
**Endpoint**: `GET /:short`  
**Description**: Redirect a short link to its original URL. Tracks access count.

**Example (cURL)**:
```bash
curl -L http://localhost:8080/example
```

---

### 3. Fetch All URLs
**Endpoint**: `GET /urls`  
**Description**: Retrieve a list of all shortened URLs, including access counts and expiration dates.

**Response**:
```json
[
  {
    "id": 1,
    "original": "https://www.example.com",
    "short": "example",
    "qr_code": "qrcodes/example.png",
    "custom_short": "example",
    "expiration_date": "2025-04-30T23:59:59Z",
    "access_count": 5
  }
]
```

**Example (cURL)**:
```bash
curl http://localhost:8080/urls
```

---

### 4. QR Code Access
**QR Code Path**: `qrcodes/{short}.png`  
**Description**: Access the QR code image generated for a short link.

**Example (cURL)**:
```bash
curl http://localhost:8080/qrcodes/example.png
```

---

## Implementation Details

### Database Schema
The SQLite database stores the following fields in the `urls` table:
- `id` (INTEGER): Primary key.
- `original` (TEXT): The original long URL.
- `short` (TEXT): The generated or custom short link.
- `qr_code` (TEXT): Path to the QR code image.
- `custom_short` (TEXT): User-specified custom short link.
- `expiration_date` (DATETIME): Optional expiration date for the short link.
- `access_count` (INTEGER): Tracks the number of times the short link is accessed.

### Redis Caching
Redis is used to cache short links for faster lookups. If a short link is not found in Redis, it is retrieved from SQLite and then cached in Redis.

---

## Code Walkthrough

### 1. `shortenURL` Function
Handles `POST /shorten` requests:
- Validates the request body.
- Generates a random alphanumeric short link or uses the custom short link if provided.
- Generates a QR code for the short link.
- Saves the data in SQLite and caches the short link in Redis.

### 2. `redirectURL` Function
Handles `GET /:short` requests:
- Checks Redis for the short link.
- If not found, retrieves the original URL from SQLite.
- Increments the access count in SQLite.
- Redirects the user to the original URL.

### 3. `getURLs` Function
Handles `GET /urls` requests:
- Retrieves all URLs from SQLite.
- Returns the data as a JSON array.

### 4. Utility Functions
- **`generateShortLink`**: Generates a random alphanumeric string of length 6.
- **`stringWithCharset`**: Helper function to generate random strings.
- **`validate`**: Ensures request body fields adhere to required formats (e.g., valid URL).

---

## Directory Structure
```
shorrt/
├── main.go         # Application entry point
├── urlshortener.db # SQLite database file
├── qrcodes/        # Directory to store QR code images
├── index.html      # Static web interface
└── README.md       # Documentation
```

---

## How to Run

### Prerequisites
- Go (1.18+)
- Redis
- SQLite

### Setup
1. Clone the repository:
   ```bash
   git clone https://github.com/your-repo/shorrt.git
   cd shorrt
   ```

2. Install dependencies:
   ```bash
   go mod tidy
   ```

3. Start Redis:
   ```bash
   redis-server
   ```

4. Run the application:
   ```bash
   go run main.go
   ```

### Access the Web Interface
Open `http://localhost:8080/` in your browser to use the web interface for managing short links.

---

## Future Enhancements
1. **User Authentication**: Allow users to log in and manage their own short links.
2. **Custom QR Code Sizes**: Let users generate QR codes with custom dimensions.
3. **Analytics Dashboard**: Provide a comprehensive dashboard for viewing access trends.

---

## License
This project is licensed under the MIT License. See the `LICENSE` file for details.
