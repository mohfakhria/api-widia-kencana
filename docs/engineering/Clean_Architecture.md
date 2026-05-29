# Clean Architecture — Transport-Agnostic Golang

> Adaptasi **The Clean Architecture** (Uncle Bob) untuk aplikasi Go yang dapat melayani
> berbagai transport protocol secara bersamaan: HTTP REST, gRPC, TCP, WebSocket, CLI, Message Queue, dan lainnya.

---

## Daftar Isi

1. [Prinsip Inti](#1-prinsip-inti)
2. [The Dependency Rule](#2-the-dependency-rule)
3. [Model Mental: Port & Adapter](#3-model-mental-port--adapter)
4. [Lapisan Arsitektur](#4-lapisan-arsitektur)
5. [Struktur Direktori](#5-struktur-direktori)
6. [Kontrak Antar Lapisan (Interfaces)](#6-kontrak-antar-lapisan-interfaces)
7. [Implementasi Per Lapisan](#7-implementasi-per-lapisan)
8. [Wiring: Dependency Injection](#8-wiring-dependency-injection)
9. [Menambah Transport Baru](#9-menambah-transport-baru)
10. [Pola Data Transfer](#10-pola-data-transfer)
11. [Error Handling Lintas Transport](#11-error-handling-lintas-transport)
12. [Testing Strategy](#12-testing-strategy)
13. [Aturan Wajib](#13-aturan-wajib)
14. [Checklist Kepatuhan](#14-checklist-kepatuhan)

---

## 1. Prinsip Inti

Masalah utama ketika sebuah aplikasi hanya di-design untuk HTTP adalah **coupling antara protokol dan logika bisnis**. Ketika perlu menambah gRPC atau TCP, kode use case ikut berubah — ini adalah pelanggaran Clean Architecture.

**Solusinya:** Use case tidak pernah tahu melalui "pintu" mana request masuk. Use case hanya tahu:
- Menerima **input berupa domain object** (bukan HTTP request, bukan protobuf message)
- Mengembalikan **output berupa domain object** (bukan HTTP response, bukan JSON byte)

Transport apapun yang ingin digunakan tinggal membungkus use case yang sama.

```
┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐
│   HTTP   │  │   gRPC   │  │   TCP    │  │WebSocket │  │   CLI    │  ← Transport
└────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘
     │              │              │              │              │
     └──────────────┴──────────────┴──────────────┴─────────────┘
                                   │
                                   ▼
                         ┌─────────────────┐
                         │   Use Case API  │  ← Satu interface, semua transport pakai
                         └────────┬────────┘
                                  │
                         ┌────────▼────────┐
                         │    Entities     │  ← Business rules murni
                         └─────────────────┘
```

---

## 2. The Dependency Rule

> *"Source code dependencies hanya boleh menunjuk ke DALAM. Tidak ada di lingkaran dalam yang boleh mengetahui apapun tentang lingkaran luar."*

```
╔══════════════════════════════════════════════════════════════════╗
║  FRAMEWORKS & DRIVERS                                            ║
║   (HTTP Server, gRPC Server, TCP Listener, DB Driver, MQ Client) ║
║  ╔════════════════════════════════════════════════════════════╗  ║
║  ║  INTERFACE ADAPTERS                                        ║  ║
║  ║   (Delivery: handlers, consumers, listeners)               ║  ║
║  ║   (Persistence: repository implementations)                ║  ║
║  ║  ╔══════════════════════════════════════════════════════╗  ║  ║
║  ║  ║  USE CASES                                           ║  ║  ║
║  ║  ║   (Application business rules)                       ║  ║  ║
║  ║  ║   (Port interfaces defined here)                     ║  ║  ║
║  ║  ║  ╔════════════════════════════════════════════════╗  ║  ║  ║
║  ║  ║  ║  ENTITIES                                      ║  ║  ║  ║
║  ║  ║  ║   (Domain models, domain rules, domain errors) ║  ║  ║  ║
║  ║  ║  ╚════════════════════════════════════════════════╝  ║  ║  ║
║  ║  ╚══════════════════════════════════════════════════════╝  ║  ║
║  ╚════════════════════════════════════════════════════════════╝  ║
╚══════════════════════════════════════════════════════════════════╝

         Arah dependensi source code:  ──────────────────▶  DALAM
```

---

## 3. Model Mental: Port & Adapter

Arsitektur ini mengadopsi konsep **Hexagonal Architecture** (Ports & Adapters) yang sejalan dengan Clean Architecture Uncle Bob:

```
                    ┌──────────────────────────────┐
                    │         APPLICATION           │
                    │                               │
  HTTP ────────────▶│  [Driving Port]               │
  gRPC ────────────▶│  (Input Port / Use Case API)  │◀─── Driving Adapters
  TCP  ────────────▶│                               │     (yang memulai interaksi)
  CLI  ────────────▶│                               │
                    │                               │
                    │  [Driven Port]                │──────▶ DB (PostgreSQL)
                    │  (Output Port / Repository)   │──────▶ Cache (Redis)
                    │                               │──────▶ MQ (Kafka/RabbitMQ)
                    │                               │──────▶ External API
                    └──────────────────────────────┘
                              Driven Adapters
                         (yang digerakkan oleh app)
```

**Driving Port (Input Port):** Interface use case — didefinisikan di layer use case, diimplementasikan oleh use case, dipanggil oleh delivery layer.

**Driven Port (Output Port):** Interface repository/service — didefinisikan di layer use case, diimplementasikan oleh infrastructure, dipanggil oleh use case.

---

## 4. Lapisan Arsitektur

### Layer 1 — Entities (Domain)

- Pure Go struct, zero external import
- Domain business rules dan validasi
- Sentinel errors domain
- Value objects dan domain events
- Tidak berubah karena perubahan transport atau database

### Layer 2 — Use Cases (Application)

- Application-specific business rules
- Mendefinisikan **semua interface** yang dibutuhkan (input port & output port)
- Tidak tahu HTTP, gRPC, TCP, database, atau framework apapun
- Hanya bicara dalam bahasa domain (entity)
- Berubah hanya jika logika bisnis berubah

### Layer 3 — Interface Adapters (Delivery & Persistence)

**Delivery (Driving Adapters):**
- Menerjemahkan pesan masuk dari transport → domain input
- Memanggil use case
- Menerjemahkan domain output → format transport

**Persistence (Driven Adapters):**
- Mengimplementasikan repository interface dari layer 2
- Semua query SQL, Redis command, dsb. ada di sini

### Layer 4 — Frameworks & Drivers (Infrastructure)

- Setup server (HTTP, gRPC, TCP listener)
- Koneksi database, message queue
- Konfigurasi, logging, metrics
- Glue code — sesedikit mungkin logika

---

## 5. Struktur Direktori

```
project-root/
│
├── cmd/                                     # Entrypoints — satu per mode deployment
│   ├── api/
│   │   └── main.go                          # Signal handling + bootstrap.NewApp()
│   ├── worker/
│   │   └── main.go                          # Jalankan consumer (MQ worker)
│   └── cli/
│       └── main.go                          # CLI tool
│
├── bootstrap/                               # App assembly — satu file per binary
│   ├── shared.go                            # Shared ringan: hanya logger & config
│   ├── apiserver.go                         # Wiring API server (HTTP + gRPC + persistence)
│   ├── ouchclient.go                        # Wiring OUCH TCP client
│   └── worker.go                            # Wiring MQ consumer worker
│
├── internal/
│   │
│   ├── domain/                              # ══ LAYER 1: Entities ══
│   │   ├── entity/
│   │   │   ├── user.go                      #   Domain model + business methods
│   │   │   ├── order.go
│   │   │   └── product.go
│   │   ├── valueobject/
│   │   │   ├── email.go                     #   Value objects (immutable)
│   │   │   ├── money.go
│   │   │   └── phone.go
│   │   └── errors.go                        #   Sentinel domain errors
│   │
│   ├── usecase/                             # ══ LAYER 2: Use Cases ══
│   │   ├── port/
│   │   │   ├── input/                       #   Driving Ports (Input Ports)
│   │   │   │   ├── user_port.go             #     Interface use case (dipanggil delivery)
│   │   │   │   └── order_port.go
│   │   │   └── output/                      #   Driven Ports (Output Ports)
│   │   │       ├── user_repository.go       #     Interface repo (diimplementasi persistence)
│   │   │       ├── order_repository.go
│   │   │       ├── cache_port.go            #     Interface cache
│   │   │       ├── event_publisher.go       #     Interface event/message publisher
│   │   │       └── notifier_port.go         #     Interface notifikasi (email, SMS, push)
│   │   ├── user_usecase.go                  #   Implementasi UserUseCase
│   │   └── order_usecase.go
│   │
│   ├── delivery/                            # ══ LAYER 3: Interface Adapters (Driving) ══
│   │   ├── http/
│   │   │   ├── handler/
│   │   │   │   ├── user_handler.go
│   │   │   │   └── order_handler.go
│   │   │   ├── middleware/
│   │   │   │   ├── auth.go
│   │   │   │   └── logging.go
│   │   │   ├── dto/
│   │   │   │   ├── request/
│   │   │   │   │   └── user_request.go
│   │   │   │   └── response/
│   │   │   │       └── user_response.go
│   │   │   └── router.go
│   │   ├── grpc/
│   │   │   ├── handler/
│   │   │   │   ├── user_handler.go          #   gRPC server handler
│   │   │   │   └── order_handler.go
│   │   │   └── interceptor/
│   │   │       ├── auth.go
│   │   │       └── logging.go
│   │   ├── tcp/
│   │   │   ├── handler/
│   │   │   │   └── session_handler.go       #   TCP connection handler
│   │   │   └── codec/
│   │   │       └── json_codec.go            #   Encode/decode TCP message
│   │   ├── websocket/
│   │   │   ├── handler/
│   │   │   │   └── ws_handler.go
│   │   │   └── hub.go                       #   Connection hub
│   │   └── consumer/
│   │       ├── kafka/
│   │       │   └── order_consumer.go        #   Kafka message consumer
│   │       └── rabbitmq/
│   │           └── notification_consumer.go
│   │
│   ├── persistence/                         # ══ LAYER 3: Interface Adapters (Driven) ══
│   │   ├── postgres/
│   │   │   ├── user_repository.go           #   Implementasi UserRepository
│   │   │   └── order_repository.go
│   │   ├── redis/
│   │   │   └── cache_adapter.go             #   Implementasi CachePort
│   │   └── kafka/
│   │       └── event_publisher.go           #   Implementasi EventPublisher
│   │
│   └── infrastructure/                      # ══ LAYER 4: Frameworks & Drivers ══
│       ├── config/
│       │   └── config.go
│       ├── database/
│       │   └── postgres.go
│       ├── cache/
│       │   └── redis.go
│       ├── messaging/
│       │   ├── kafka.go
│       │   └── rabbitmq.go
│       ├── server/
│       │   ├── http.go                      #   HTTP server setup
│       │   ├── grpc.go                      #   gRPC server setup
│       │   └── tcp.go                       #   TCP listener setup
│       ├── lifecycle/
│       │   ├── runnable.go                  #   Interface Runnable
│       │   └── runner.go                    #   Runner: jalankan semua Runnable + graceful shutdown
│       └── logger/
│           └── logger.go
│
├── pkg/                                     # Shared, dapat diexport
│   ├── apperror/
│   │   └── translator.go                    # Domain error → transport error code
│   └── contextkey/
│       └── keys.go                          # Context key constants
│
├── proto/                                   # Protobuf definitions
│   └── user/
│       └── user.proto
│
├── migration/
│   ├── users.sql
│   ├── quotations.sql
│   ├── quotation_sections.sql
│   ├── quotation_items.sql
│   ├── quotation_details.sql
│   └── purchase_order.sql
│
├── go.mod
├── go.sum
├── Makefile
├── Dockerfile
└── .env.example
```

---

## 6. Kontrak Antar Lapisan (Interfaces)

Semua interface didefinisikan di `internal/usecase/port/`. Ini adalah **jantung** dari transport-agnostic design.

### Input Ports (Driving Ports) — dipanggil oleh delivery layer

```go
// internal/usecase/port/input/user_port.go
package input

import (
    "context"
    "github.com/yourorg/app/internal/domain/entity"
)

// UserUseCase — interface ini yang dipanggil oleh HTTP handler, gRPC handler, TCP handler, dll.
// Tidak ada satupun parameter yang berbau HTTP, gRPC, atau protokol apapun.
type UserUseCase interface {
    Register(ctx context.Context, cmd RegisterUserCommand) (*entity.User, error)
    Login(ctx context.Context, cmd LoginCommand) (*entity.User, string, error)
    GetByID(ctx context.Context, id int64) (*entity.User, error)
    UpdateProfile(ctx context.Context, cmd UpdateProfileCommand) (*entity.User, error)
    Delete(ctx context.Context, id int64) error
}

// Command objects — input ke use case (bukan HTTP request, bukan proto message)
type RegisterUserCommand struct {
    Name     string
    Email    string
    Password string
}

type LoginCommand struct {
    Email    string
    Password string
}

type UpdateProfileCommand struct {
    UserID int64
    Name   string
    Email  string
}
```

### Output Ports (Driven Ports) — diimplementasikan oleh persistence

```go
// internal/usecase/port/output/user_repository.go
package output

import (
    "context"
    "github.com/yourorg/app/internal/domain/entity"
)

// UserRepository — output port untuk data persistence
// Tidak ada referensi ke SQL, Redis, atau storage apapun
type UserRepository interface {
    FindByID(ctx context.Context, id int64) (*entity.User, error)
    FindByEmail(ctx context.Context, email string) (*entity.User, error)
    Save(ctx context.Context, user *entity.User) (*entity.User, error)
    Update(ctx context.Context, user *entity.User) (*entity.User, error)
    Delete(ctx context.Context, id int64) error
}
```

```go
// internal/usecase/port/output/cache_port.go
package output

import (
    "context"
    "time"
)

// CachePort — output port untuk caching, tidak tahu Redis atau Memcached
type CachePort interface {
    Set(ctx context.Context, key string, value any, ttl time.Duration) error
    Get(ctx context.Context, key string, dest any) error
    Delete(ctx context.Context, key string) error
}
```

```go
// internal/usecase/port/output/event_publisher.go
package output

import "context"

// EventPublisher — output port untuk publish event/message
// Use case tidak tahu Kafka, RabbitMQ, atau NATS
type EventPublisher interface {
    Publish(ctx context.Context, topic string, event any) error
}
```

```go
// internal/usecase/port/output/notifier_port.go
package output

import "context"

// Notifier — output port untuk notifikasi ke pengguna
type Notifier interface {
    SendEmail(ctx context.Context, to, subject, body string) error
    SendSMS(ctx context.Context, phone, message string) error
}
```

---

## 7. Implementasi Per Lapisan

### Layer 1: Entity

```go
// internal/domain/entity/user.go
package entity

import "time"

type User struct {
    ID           int64
    Name         string
    Email        string
    PasswordHash string
    Active       bool
    CreatedAt    time.Time
    UpdatedAt    time.Time
}

func (u *User) Deactivate() {
    u.Active = false
}

func (u *User) IsOwner(resourceOwnerID int64) bool {
    return u.ID == resourceOwnerID
}
```

```go
// internal/domain/errors.go
package domain

import "errors"

var (
    ErrNotFound        = errors.New("not found")
    ErrAlreadyExists   = errors.New("already exists")
    ErrUnauthorized    = errors.New("unauthorized")
    ErrForbidden       = errors.New("forbidden")
    ErrInvalidInput    = errors.New("invalid input")
    ErrInternalFailure = errors.New("internal failure")
)
```

### Layer 2: Use Case

```go
// internal/usecase/user_usecase.go
package usecase

import (
    "context"
    "fmt"

    "github.com/yourorg/app/internal/domain"
    "github.com/yourorg/app/internal/domain/entity"
    "github.com/yourorg/app/internal/usecase/port/input"
    "github.com/yourorg/app/internal/usecase/port/output"
)

type userUseCase struct {
    userRepo  output.UserRepository
    cache     output.CachePort
    publisher output.EventPublisher
    notifier  output.Notifier
}

// NewUserUseCase — constructor injection
func NewUserUseCase(
    userRepo output.UserRepository,
    cache output.CachePort,
    publisher output.EventPublisher,
    notifier output.Notifier,
) input.UserUseCase {
    return &userUseCase{
        userRepo:  userRepo,
        cache:     cache,
        publisher: publisher,
        notifier:  notifier,
    }
}

func (uc *userUseCase) Register(ctx context.Context, cmd input.RegisterUserCommand) (*entity.User, error) {
    existing, _ := uc.userRepo.FindByEmail(ctx, cmd.Email)
    if existing != nil {
        return nil, domain.ErrAlreadyExists
    }

    user := &entity.User{
        Name:         cmd.Name,
        Email:        cmd.Email,
        PasswordHash: hashPassword(cmd.Password),
        Active:       true,
    }

    saved, err := uc.userRepo.Save(ctx, user)
    if err != nil {
        return nil, fmt.Errorf("save user: %w", err)
    }

    // Publish event — use case tidak peduli siapa yang konsumsi (Kafka? RabbitMQ?)
    _ = uc.publisher.Publish(ctx, "user.registered", saved)

    // Kirim notifikasi — use case tidak peduli email provider-nya apa
    _ = uc.notifier.SendEmail(ctx, saved.Email, "Welcome!", "Thanks for registering.")

    return saved, nil
}

func (uc *userUseCase) GetByID(ctx context.Context, id int64) (*entity.User, error) {
    cacheKey := fmt.Sprintf("user:%d", id)

    var user entity.User
    if err := uc.cache.Get(ctx, cacheKey, &user); err == nil {
        return &user, nil // cache hit
    }

    found, err := uc.userRepo.FindByID(ctx, id)
    if err != nil {
        return nil, domain.ErrNotFound
    }

    _ = uc.cache.Set(ctx, cacheKey, found, 0)
    return found, nil
}

func hashPassword(plain string) string { return plain /* bcrypt in real impl */ }
```

### Layer 3: Delivery — HTTP Handler

```go
// internal/delivery/http/handler/user_handler.go
package handler

import (
    "encoding/json"
    "errors"
    "net/http"
    "strconv"

    "github.com/go-chi/chi/v5"
    "github.com/yourorg/app/internal/domain"
    httpdto "github.com/yourorg/app/internal/delivery/http/dto"
    "github.com/yourorg/app/internal/usecase/port/input"
)

type UserHTTPHandler struct {
    uc input.UserUseCase // ← Hanya tahu interface, tidak peduli implementasinya
}

func NewUserHTTPHandler(uc input.UserUseCase) *UserHTTPHandler {
    return &UserHTTPHandler{uc: uc}
}

func (h *UserHTTPHandler) Register(w http.ResponseWriter, r *http.Request) {
    var req httpdto.RegisterRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "invalid body")
        return
    }

    // HTTP DTO → Use Case Command (bukan entity)
    cmd := input.RegisterUserCommand{
        Name:     req.Name,
        Email:    req.Email,
        Password: req.Password,
    }

    user, err := h.uc.Register(r.Context(), cmd)
    if err != nil {
        // Terjemahkan domain error → HTTP status
        switch {
        case errors.Is(err, domain.ErrAlreadyExists):
            writeError(w, http.StatusConflict, err.Error())
        case errors.Is(err, domain.ErrInvalidInput):
            writeError(w, http.StatusBadRequest, err.Error())
        default:
            writeError(w, http.StatusInternalServerError, "internal error")
        }
        return
    }

    // Entity → HTTP Response DTO
    writeJSON(w, http.StatusCreated, httpdto.UserFromEntity(user))
}

func (h *UserHTTPHandler) GetByID(w http.ResponseWriter, r *http.Request) {
    id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

    user, err := h.uc.GetByID(r.Context(), id)
    if err != nil {
        writeError(w, http.StatusNotFound, err.Error())
        return
    }

    writeJSON(w, http.StatusOK, httpdto.UserFromEntity(user))
}
```

### Layer 3: Delivery — gRPC Handler

```go
// internal/delivery/grpc/handler/user_handler.go
package handler

import (
    "context"
    "errors"

    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
    "github.com/yourorg/app/internal/domain"
    pb "github.com/yourorg/app/proto/user"
    "github.com/yourorg/app/internal/usecase/port/input"
)

type UserGRPCHandler struct {
    pb.UnimplementedUserServiceServer
    uc input.UserUseCase // ← Interface yang SAMA dengan HTTP handler
}

func NewUserGRPCHandler(uc input.UserUseCase) *UserGRPCHandler {
    return &UserGRPCHandler{uc: uc}
}

func (h *UserGRPCHandler) RegisterUser(ctx context.Context, req *pb.RegisterRequest) (*pb.UserResponse, error) {
    // Proto message → Use Case Command
    cmd := input.RegisterUserCommand{
        Name:     req.Name,
        Email:    req.Email,
        Password: req.Password,
    }

    user, err := h.uc.Register(ctx, cmd)
    if err != nil {
        // Terjemahkan domain error → gRPC status code
        switch {
        case errors.Is(err, domain.ErrAlreadyExists):
            return nil, status.Error(codes.AlreadyExists, err.Error())
        case errors.Is(err, domain.ErrInvalidInput):
            return nil, status.Error(codes.InvalidArgument, err.Error())
        default:
            return nil, status.Error(codes.Internal, "internal error")
        }
    }

    // Entity → Proto Response
    return &pb.UserResponse{
        Id:    user.ID,
        Name:  user.Name,
        Email: user.Email,
    }, nil
}
```

### Layer 3: Delivery — TCP Handler

```go
// internal/delivery/tcp/handler/session_handler.go
package handler

import (
    "bufio"
    "context"
    "encoding/json"
    "errors"
    "net"

    "github.com/yourorg/app/internal/domain"
    "github.com/yourorg/app/internal/usecase/port/input"
)

// TCPMessage — struktur pesan generik via TCP
type TCPMessage struct {
    Action  string          `json:"action"`
    Payload json.RawMessage `json:"payload"`
}

type TCPResponse struct {
    Status  string `json:"status"`
    Payload any    `json:"payload,omitempty"`
    Error   string `json:"error,omitempty"`
}

type UserTCPHandler struct {
    uc input.UserUseCase // ← Interface yang SAMA
}

func NewUserTCPHandler(uc input.UserUseCase) *UserTCPHandler {
    return &UserTCPHandler{uc: uc}
}

// Handle — menangani satu koneksi TCP
func (h *UserTCPHandler) Handle(conn net.Conn) {
    defer conn.Close()
    scanner := bufio.NewScanner(conn)
    enc := json.NewEncoder(conn)

    for scanner.Scan() {
        var msg TCPMessage
        if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
            enc.Encode(TCPResponse{Status: "error", Error: "invalid message"})
            continue
        }
        enc.Encode(h.dispatch(msg))
    }
}

func (h *UserTCPHandler) dispatch(msg TCPMessage) TCPResponse {
    ctx := context.Background()

    switch msg.Action {
    case "user.register":
        var cmd input.RegisterUserCommand
        json.Unmarshal(msg.Payload, &cmd)

        user, err := h.uc.Register(ctx, cmd)
        if err != nil {
            return h.domainErrToTCP(err)
        }
        return TCPResponse{Status: "ok", Payload: user}

    case "user.get":
        var req struct {
            ID int64 `json:"id"`
        }
        json.Unmarshal(msg.Payload, &req)

        user, err := h.uc.GetByID(ctx, req.ID)
        if err != nil {
            return h.domainErrToTCP(err)
        }
        return TCPResponse{Status: "ok", Payload: user}

    default:
        return TCPResponse{Status: "error", Error: "unknown action"}
    }
}

func (h *UserTCPHandler) domainErrToTCP(err error) TCPResponse {
    switch {
    case errors.Is(err, domain.ErrNotFound):
        return TCPResponse{Status: "not_found", Error: err.Error()}
    case errors.Is(err, domain.ErrAlreadyExists):
        return TCPResponse{Status: "conflict", Error: err.Error()}
    default:
        return TCPResponse{Status: "error", Error: "internal error"}
    }
}
```

### Layer 3: Delivery — Message Queue Consumer

```go
// internal/delivery/consumer/kafka/order_consumer.go
package kafka

import (
    "context"
    "encoding/json"
    "log/slog"

    "github.com/yourorg/app/internal/usecase/port/input"
)

type OrderConsumer struct {
    orderUC input.OrderUseCase // ← Interface yang SAMA
    logger  *slog.Logger
}

func NewOrderConsumer(orderUC input.OrderUseCase, logger *slog.Logger) *OrderConsumer {
    return &OrderConsumer{orderUC: orderUC, logger: logger}
}

// HandleMessage — dipanggil oleh Kafka consumer loop di infrastructure layer
func (c *OrderConsumer) HandleMessage(ctx context.Context, topic string, data []byte) error {
    switch topic {
    case "payment.completed":
        var event struct {
            OrderID int64  `json:"order_id"`
            Status  string `json:"status"`
        }
        if err := json.Unmarshal(data, &event); err != nil {
            return err
        }

        // Kafka message → Use Case Command
        return c.orderUC.ConfirmOrder(ctx, input.ConfirmOrderCommand{
            OrderID: event.OrderID,
        })
    }
    return nil
}
```

### Layer 3: Persistence — Repository Implementation

```go
// internal/persistence/postgres/user_repository.go
package postgres

import (
    "context"
    "database/sql"
    "fmt"

    "github.com/yourorg/app/internal/domain"
    "github.com/yourorg/app/internal/domain/entity"
    "github.com/yourorg/app/internal/usecase/port/output"
)

type userPostgresRepo struct {
    db *sql.DB
}

// NewUserPostgresRepo — verifikasi compile-time bahwa struct ini memenuhi interface
func NewUserPostgresRepo(db *sql.DB) output.UserRepository {
    return &userPostgresRepo{db: db}
}

func (r *userPostgresRepo) FindByID(ctx context.Context, id int64) (*entity.User, error) {
    u := &entity.User{}
    err := r.db.QueryRowContext(ctx,
        `SELECT id, name, email, active, created_at FROM users WHERE id = $1`, id,
    ).Scan(&u.ID, &u.Name, &u.Email, &u.Active, &u.CreatedAt)

    if err == sql.ErrNoRows {
        return nil, fmt.Errorf("user %d: %w", id, domain.ErrNotFound)
    }
    return u, err
}

func (r *userPostgresRepo) Save(ctx context.Context, user *entity.User) (*entity.User, error) {
    err := r.db.QueryRowContext(ctx,
        `INSERT INTO users (name, email, password_hash, active, created_at, updated_at)
         VALUES ($1, $2, $3, $4, NOW(), NOW()) RETURNING id, created_at`,
        user.Name, user.Email, user.PasswordHash, user.Active,
    ).Scan(&user.ID, &user.CreatedAt)
    return user, err
}

// ... FindByEmail, Update, Delete mengikuti pola yang sama
```

### Layer 3: Persistence — Cache Adapter

```go
// internal/persistence/redis/cache_adapter.go
package redis

import (
    "context"
    "encoding/json"
    "time"

    "github.com/redis/go-redis/v9"
    "github.com/yourorg/app/internal/usecase/port/output"
)

type redisCacheAdapter struct {
    client *redis.Client
}

func NewRedisCacheAdapter(client *redis.Client) output.CachePort {
    return &redisCacheAdapter{client: client}
}

func (c *redisCacheAdapter) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
    data, _ := json.Marshal(value)
    return c.client.Set(ctx, key, data, ttl).Err()
}

func (c *redisCacheAdapter) Get(ctx context.Context, key string, dest any) error {
    data, err := c.client.Get(ctx, key).Bytes()
    if err != nil {
        return err
    }
    return json.Unmarshal(data, dest)
}

func (c *redisCacheAdapter) Delete(ctx context.Context, key string) error {
    return c.client.Del(ctx, key).Err()
}
```

---

## 8. Wiring: Dependency Injection

Setiap binary memiliki bootstrap file-nya sendiri. Tidak ada `app.go` yang monolitik — tiap file hanya tahu apa yang dibutuhkan binary tersebut.

| File | Dipakai oleh | Isi wiring |
|---|---|---|
| `bootstrap/shared.go` | Semua binary | Hanya logger & config |
| `bootstrap/apiserver.go` | `cmd/api/` | DB, cache, HTTP/gRPC server |
| `bootstrap/ouchclient.go` | `cmd/ouchclient/` | TCP connection, OUCH protocol |
| `bootstrap/worker.go` | `cmd/worker/` | MQ consumer, DB |

### bootstrap/shared.go — hanya logger & config

`shared` hanya berisi apa yang benar-benar dipakai semua binary. DB dan cache **tidak** ada di sini — masing-masing binary meinstansiasi resource yang memang ia butuhkan.

```go
// bootstrap/shared.go
package bootstrap

import (
    "log/slog"
    "os"

    "github.com/yourorg/app/internal/infrastructure/config"
)

// shared — resource minimal yang relevan untuk semua binary
type shared struct {
    cfg    *config.Config
    logger *slog.Logger
}

func newShared() (*shared, error) {
    cfg, err := config.Load()
    if err != nil {
        return nil, fmt.Errorf("load config: %w", err)
    }

    logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    }))

    return &shared{
        cfg:    cfg,
        logger: logger,
    }, nil
}
```

### bootstrap/apiserver.go — API server binary

```go
// bootstrap/apiserver.go
package bootstrap

import (
    "context"
    "database/sql"
    "log/slog"

    _ "github.com/lib/pq"
    "github.com/redis/go-redis/v9"

    pgpersist  "github.com/yourorg/app/internal/persistence/postgres"
    rdspersist "github.com/yourorg/app/internal/persistence/redis"
    httpserver  "github.com/yourorg/app/internal/infrastructure/server"
    "github.com/yourorg/app/internal/bootstrap"
    "github.com/yourorg/app/internal/usecase"
)

type ApiServerApp struct {
    Context       context.Context
    ServiceLogger *slog.Logger
    Runnables     []lifecycle.Runnable

    shared *shared
    db     *sql.DB
    rdb    *redis.Client
}

func NewApiServerApp(ctx context.Context) *ApiServerApp {
    return &ApiServerApp{
        Context:       ctx,
        ServiceLogger: slog.Default(),
    }
}

func (a *ApiServerApp) initialize() error {
    s, err := newShared()
    if err != nil {
        return err
    }
    a.shared = s
    a.ServiceLogger = s.logger

    // ── Infrastruktur spesifik API server ────────────────────────────
    db, err := sql.Open("postgres", s.cfg.DatabaseURL)
    if err != nil {
        return fmt.Errorf("open db: %w", err)
    }
    if err := db.PingContext(a.Context); err != nil {
        return fmt.Errorf("ping db: %w", err)
    }
    a.db = db

    rdb := redis.NewClient(&redis.Options{Addr: s.cfg.RedisAddr})
    a.rdb = rdb

    // ── Driven Adapters ──────────────────────────────────────────────
    userRepo := pgpersist.NewUserPostgresRepo(db)
    cache    := rdspersist.NewRedisCacheAdapter(rdb)

    // ── Use Cases ────────────────────────────────────────────────────
    userUC := usecase.NewUserUseCase(userRepo, cache)

    // ── Runnables ────────────────────────────────────────────────────
    a.Runnables = []lifecycle.Runnable{
        httpserver.NewHTTPServer(s.cfg, userUC, s.logger),
        httpserver.NewGRPCServer(s.cfg, userUC, s.logger),
    }

    return nil
}

func (a *ApiServerApp) Start() error {
    if err := a.initialize(); err != nil {
        return err
    }
    defer a.Cleanup()

    runner := lifecycle.NewRunner(a.ServiceLogger)
    return runner.Run(a.Context, a.Runnables)
}

func (a *ApiServerApp) Cleanup() {
    if a.rdb != nil {
        a.rdb.Close()
    }
    if a.db != nil {
        a.db.Close()
    }
}
```

### bootstrap/ouchclient.go — OUCH TCP client binary

```go
// bootstrap/ouchclient.go
package bootstrap

import (
    "context"
    "log/slog"

    "github.com/yourorg/app/internal/bootstrap"
    "github.com/yourorg/app/internal/infrastructure/server"
    "github.com/yourorg/app/internal/usecase"
)

// OuchClientApp — binary khusus OUCH TCP protocol
// Tidak perlu DB atau cache — hanya TCP connection & use case spesifik trading
type OuchClientApp struct {
    Context       context.Context
    ServiceLogger *slog.Logger
    Runnables     []lifecycle.Runnable

    shared *shared
}

func NewOuchClientApp(ctx context.Context) *OuchClientApp {
    return &OuchClientApp{
        Context:       ctx,
        ServiceLogger: slog.Default(),
    }
}

func (a *OuchClientApp) initialize() error {
    s, err := newShared()
    if err != nil {
        return err
    }
    a.shared = s
    a.ServiceLogger = s.logger

    // ── Use Cases spesifik OUCH ───────────────────────────────────────
    // Tidak ada DB — state di-manage in-memory atau lewat TCP session
    orderUC := usecase.NewOrderUseCase()

    // ── Runnables ────────────────────────────────────────────────────
    a.Runnables = []lifecycle.Runnable{
        server.NewOuchTCPClient(s.cfg, orderUC, s.logger),
    }

    return nil
}

func (a *OuchClientApp) Start() error {
    if err := a.initialize(); err != nil {
        return err
    }
    defer a.Cleanup()

    runner := lifecycle.NewRunner(a.ServiceLogger)
    return runner.Run(a.Context, a.Runnables)
}

func (a *OuchClientApp) Cleanup() {
    // Tidak ada resource eksternal untuk ditutup — TCP client ditutup via ctx
}
```

### bootstrap/worker.go — MQ consumer binary

```go
// bootstrap/worker.go
package bootstrap

import (
    "context"
    "database/sql"
    "log/slog"

    _ "github.com/lib/pq"

    pgpersist "github.com/yourorg/app/internal/persistence/postgres"
    kafkapersist "github.com/yourorg/app/internal/persistence/kafka"
    "github.com/yourorg/app/internal/bootstrap"
    "github.com/yourorg/app/internal/usecase"
)

// WorkerApp — binary khusus untuk consume message dari MQ
// Butuh DB untuk persist, tapi tidak butuh HTTP/gRPC server
type WorkerApp struct {
    Context       context.Context
    ServiceLogger *slog.Logger
    Runnables     []lifecycle.Runnable

    shared *shared
    db     *sql.DB
}

func NewWorkerApp(ctx context.Context) *WorkerApp {
    return &WorkerApp{
        Context:       ctx,
        ServiceLogger: slog.Default(),
    }
}

func (a *WorkerApp) initialize() error {
    s, err := newShared()
    if err != nil {
        return err
    }
    a.shared = s
    a.ServiceLogger = s.logger

    // ── Infrastruktur spesifik worker ────────────────────────────────
    db, err := sql.Open("postgres", s.cfg.DatabaseURL)
    if err != nil {
        return fmt.Errorf("open db: %w", err)
    }
    a.db = db

    // ── Driven Adapters ──────────────────────────────────────────────
    orderRepo := pgpersist.NewOrderPostgresRepo(db)
    publisher := kafkapersist.NewKafkaEventPublisher(s.cfg.KafkaBrokers)

    // ── Use Cases ────────────────────────────────────────────────────
    orderUC := usecase.NewOrderUseCase(orderRepo, publisher)

    // ── Runnables: setiap consumer topic = satu Runnable ─────────────
    a.Runnables = []lifecycle.Runnable{
        kafkapersist.NewPaymentConsumer(s.cfg, orderUC, s.logger),
        kafkapersist.NewRefundConsumer(s.cfg, orderUC, s.logger),
    }

    return nil
}

func (a *WorkerApp) Start() error {
    if err := a.initialize(); err != nil {
        return err
    }
    defer a.Cleanup()

    runner := lifecycle.NewRunner(a.ServiceLogger)
    return runner.Run(a.Context, a.Runnables)
}

func (a *WorkerApp) Cleanup() {
    if a.db != nil {
        a.db.Close()
    }
}
```

### cmd per binary — masing-masing pakai bootstrap file-nya sendiri

```go
// cmd/api/main.go
package main

import (
    "context"
    "os"
    "os/signal"
    "syscall"

    "github.com/yourorg/app/bootstrap"
)

func main() {
    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    app := bootstrap.NewApiServerApp(ctx)

    if err := app.Start(); err != nil {
        app.ServiceLogger.Error("apiserver exited with error", "error", err)
        os.Exit(1)
    }

    app.ServiceLogger.Info("apiserver shutdown complete")
}
```

```go
// cmd/ouchclient/main.go
package main

import (
    "context"
    "os"
    "os/signal"
    "syscall"

    "github.com/yourorg/app/bootstrap"
)

func main() {
    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    app := bootstrap.NewOuchClientApp(ctx)

    if err := app.Start(); err != nil {
        app.ServiceLogger.Error("ouchclient exited with error", "error", err)
        os.Exit(1)
    }

    app.ServiceLogger.Info("ouchclient shutdown complete")
}
```

```go
// cmd/worker/main.go
package main

import (
    "context"
    "os"
    "os/signal"
    "syscall"

    "github.com/yourorg/app/bootstrap"
)

func main() {
    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    app := bootstrap.NewWorkerApp(ctx)

    if err := app.Start(); err != nil {
        app.ServiceLogger.Error("worker exited with error", "error", err)
        os.Exit(1)
    }

    app.ServiceLogger.Info("worker shutdown complete")
}
```

Pola setiap `main.go` identik — yang berbeda hanya nama bootstrap yang dipanggil. Tidak ada satu baris logika bisnis atau wiring infrastructure yang bocor ke `cmd/`.

### internal/bootstrap — Runnable & Runner

```go
// internal/bootstrap/runnable.go
package lifecycle

import "context"

// Runnable — kontrak untuk semua komponen yang dapat dijalankan dan dihentikan
// HTTP server, gRPC server, TCP listener, MQ consumer — semua mengimplementasikan ini
type Runnable interface {
    // Run memulai komponen dan memblokir hingga ctx selesai atau terjadi error
    Run(ctx context.Context) error
    // Name mengembalikan nama komponen untuk logging
    Name() string
}
```

```go
// internal/bootstrap/runner.go
package lifecycle

import (
    "context"
    "fmt"
    "log/slog"
    "sync"
)

// Runner — menjalankan semua Runnable secara concurrent dan menunggu semuanya selesai
type Runner struct {
    logger *slog.Logger
}

func NewRunner(logger *slog.Logger) *Runner {
    return &Runner{logger: logger}
}

func (r *Runner) Run(ctx context.Context, runnables []Runnable) error {
    if len(runnables) == 0 {
        r.logger.Warn("no runnables registered")
        return nil
    }

    var wg sync.WaitGroup
    errCh := make(chan error, len(runnables))

    for _, runnable := range runnables {
        wg.Add(1)
        go func(rb Runnable) {
            defer wg.Done()
            r.logger.Info("starting", "component", rb.Name())
            if err := rb.Run(ctx); err != nil {
                r.logger.Error("component stopped with error", "component", rb.Name(), "error", err)
                errCh <- fmt.Errorf("%s: %w", rb.Name(), err)
                return
            }
            r.logger.Info("component stopped gracefully", "component", rb.Name())
        }(runnable)
    }

    // Tunggu semua selesai
    wg.Wait()
    close(errCh)

    // Kumpulkan semua error
    var errs []error
    for err := range errCh {
        errs = append(errs, err)
    }

    if len(errs) > 0 {
        return errors.Join(errs...)
    }
    return nil
}
```

### Contoh implementasi Runnable — HTTP Server

```go
// internal/infrastructure/server/http.go
package server

import (
    "context"
    "errors"
    "log/slog"
    "net/http"

    "github.com/yourorg/app/internal/delivery/http/handler"
    "github.com/yourorg/app/internal/infrastructure/config"
    "github.com/yourorg/app/internal/bootstrap"
    "github.com/yourorg/app/internal/usecase/port/input"
)

type HTTPServer struct {
    server *http.Server
    logger *slog.Logger
}

// NewHTTPServer — memenuhi lifecycle.Runnable
func NewHTTPServer(cfg *config.Config, userUC input.UserUseCase, logger *slog.Logger) lifecycle.Runnable {
    userHandler := handler.NewUserHTTPHandler(userUC)
    router := setupRouter(userHandler)

    return &HTTPServer{
        server: &http.Server{Addr: cfg.HTTPAddr, Handler: router},
        logger: logger,
    }
}

func (s *HTTPServer) Name() string { return "http-server" }

func (s *HTTPServer) Run(ctx context.Context) error {
    errCh := make(chan error, 1)

    go func() {
        s.logger.Info("HTTP server listening", "addr", s.server.Addr)
        if err := s.server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
            errCh <- err
        }
    }()

    select {
    case <-ctx.Done():
        // Context cancelled — graceful shutdown
        return s.server.Shutdown(context.Background())
    case err := <-errCh:
        return err
    }
}
```

Pola `Run(ctx)` yang sama dipakai untuk gRPC server, TCP listener, dan MQ consumer — semuanya mengimplementasikan `lifecycle.Runnable`.

---

## 9. Menambah Transport atau Binary Baru

### Tambah transport baru ke binary yang sudah ada

Untuk menambah transport baru ke binary yang sudah ada (misal: tambah WebSocket ke `apiserver`), **tidak ada satupun baris kode di `usecase/` atau `domain/` yang perlu diubah**.

```
1. Buat folder  internal/delivery/websocket/
2. Implementasikan lifecycle.Runnable di internal/infrastructure/server/ws.go
3. Terjemahkan WebSocket message → Command (bukan entity)
4. Terjemahkan entity result → WebSocket message
5. Tambahkan ke a.Runnables di bootstrap/apiserver.go
```

### Tambah binary baru

```
1. Buat cmd/<nama>/main.go      — hanya signal + bootstrap.New*App(ctx) + app.Start()
2. Buat bootstrap/<nama>.go     — struct App + initialize() + Start() + Cleanup()
3. initialize() hanya instansiasi resource yang binary itu butuhkan
4. Tidak ada perubahan di usecase/, domain/, atau bootstrap/shared.go
```

Tabel perbandingan ketiga binary:

| | `apiserver` | `ouchclient` | `worker` |
|---|---|---|---|
| **DB** | ✅ PostgreSQL | ❌ tidak perlu | ✅ PostgreSQL |
| **Cache** | ✅ Redis | ❌ tidak perlu | ❌ tidak perlu |
| **Transport** | HTTP + gRPC | TCP (OUCH) | MQ consumer |
| **Use case** | UserUC, OrderUC | OrderUC (trading) | OrderUC |
| **shared** | logger + config | logger + config | logger + config |

Tabel perbandingan tugas masing-masing handler untuk operasi yang **sama**:

| Operasi          | HTTP Handler          | gRPC Handler          | TCP Handler              | MQ Consumer          |
|------------------|-----------------------|-----------------------|--------------------------|----------------------|
| **Parse input**  | `json.Decode(body)`   | Proto message field   | `json.Unmarshal(bytes)`  | `json.Unmarshal(msg)`|
| **Buat command** | `RegisterUserCommand` | `RegisterUserCommand` | `RegisterUserCommand`    | `ConfirmOrderCommand`|
| **Panggil UC**   | `uc.Register(ctx, cmd)` | `uc.Register(ctx, cmd)` | `uc.Register(ctx, cmd)` | `uc.ConfirmOrder(ctx, cmd)` |
| **Format output**| JSON HTTP response    | Proto response        | JSON TCP response        | — (fire and forget)  |
| **Map error**    | HTTP status code      | gRPC status code      | TCP status string        | retry / DLQ          |

Use case yang dipanggil **identik** di semua transport. Hanya adaptor bungkus terluarnya yang berbeda.

---

## 10. Pola Data Transfer

```
┌─────────────────────────────────────────────────────────────────────┐
│                         DATA FLOW                                    │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  [Transport IN]                                                      │
│   HTTP JSON / gRPC Proto / TCP Bytes / WS Message / MQ Payload      │
│         │                                                            │
│         ▼  parse & validate di delivery layer                        │
│  [Command Object]       ← bukan entity, bukan DTO transport          │
│   input.RegisterUserCommand{ Name, Email, Password }                 │
│         │                                                            │
│         ▼  use case menerima command                                 │
│  [Entity]               ← bahasa bisnis internal                     │
│   entity.User{ ID, Name, Email, Active, ... }                        │
│         │                                                            │
│         ▼  persistence menerima entity                               │
│  [Storage Format]       ← SQL row, Redis JSON, dll.                  │
│   "INSERT INTO users ..."                                            │
│                                                                      │
│  ── BALIK ARAH ──────────────────────────────────────────────────── │
│                                                                      │
│  [Entity]               ← use case kembalikan entity                 │
│         │                                                            │
│         ▼  delivery layer map ke format transport                    │
│  [Transport OUT]                                                      │
│   HTTP JSON / gRPC Proto / TCP Bytes / WS Message                   │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

**Aturan mutlak:**
- Command object mengalir dari **delivery → use case** (bukan entity, bukan transport DTO langsung)
- Entity mengalir dari **use case → delivery** dan dari/ke **persistence**
- Transport DTO (JSON struct, proto struct) **hanya hidup di delivery layer**
- Row database / Redis value **hanya hidup di persistence layer**

---

## 11. Error Handling Lintas Transport

```go
// internal/domain/errors.go — satu definisi, semua transport pakai
package domain

import "errors"

var (
    ErrNotFound        = errors.New("not found")
    ErrAlreadyExists   = errors.New("already exists")
    ErrUnauthorized    = errors.New("unauthorized")
    ErrForbidden       = errors.New("forbidden")
    ErrInvalidInput    = errors.New("invalid input")
    ErrInternalFailure = errors.New("internal failure")
)
```

```go
// pkg/apperror/translator.go — helper terjemahan per transport
package apperror

import (
    "errors"
    "net/http"

    "google.golang.org/grpc/codes"
    "github.com/yourorg/app/internal/domain"
)

func ToHTTPStatus(err error) int {
    switch {
    case errors.Is(err, domain.ErrNotFound):      return http.StatusNotFound
    case errors.Is(err, domain.ErrAlreadyExists): return http.StatusConflict
    case errors.Is(err, domain.ErrUnauthorized):  return http.StatusUnauthorized
    case errors.Is(err, domain.ErrForbidden):     return http.StatusForbidden
    case errors.Is(err, domain.ErrInvalidInput):  return http.StatusBadRequest
    default:                                       return http.StatusInternalServerError
    }
}

func ToGRPCCode(err error) codes.Code {
    switch {
    case errors.Is(err, domain.ErrNotFound):      return codes.NotFound
    case errors.Is(err, domain.ErrAlreadyExists): return codes.AlreadyExists
    case errors.Is(err, domain.ErrUnauthorized):  return codes.Unauthenticated
    case errors.Is(err, domain.ErrForbidden):     return codes.PermissionDenied
    case errors.Is(err, domain.ErrInvalidInput):  return codes.InvalidArgument
    default:                                       return codes.Internal
    }
}

func ToTCPStatus(err error) string {
    switch {
    case errors.Is(err, domain.ErrNotFound):      return "not_found"
    case errors.Is(err, domain.ErrAlreadyExists): return "conflict"
    case errors.Is(err, domain.ErrUnauthorized):  return "unauthorized"
    case errors.Is(err, domain.ErrInvalidInput):  return "bad_request"
    default:                                       return "error"
    }
}
```

---

## 12. Testing Strategy

### Piramida Test

```
         ┌──────────┐
         │   E2E    │  ← Sedikit, lambat: test server sungguhan end-to-end
         ├──────────┤
         │Integration│  ← Medium: test repository dengan testcontainers/DB test
         ├──────────┤
         │   Unit   │  ← Banyak, cepat: test use case dengan mock port
         └──────────┘
```

### Unit Test Use Case — Mock semua output port

```go
// internal/usecase/user_usecase_test.go
package usecase_test

import (
    "context"
    "errors"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
    "github.com/yourorg/app/internal/domain"
    "github.com/yourorg/app/internal/domain/entity"
    "github.com/yourorg/app/internal/usecase"
    "github.com/yourorg/app/internal/usecase/port/input"
)

// ── Mocks ──────────────────────────────────────────────────────────

type mockUserRepo struct{ mock.Mock }

func (m *mockUserRepo) FindByEmail(ctx context.Context, email string) (*entity.User, error) {
    args := m.Called(ctx, email)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*entity.User), args.Error(1)
}
func (m *mockUserRepo) Save(ctx context.Context, u *entity.User) (*entity.User, error) {
    args := m.Called(ctx, u)
    return args.Get(0).(*entity.User), args.Error(1)
}
func (m *mockUserRepo) FindByID(ctx context.Context, id int64) (*entity.User, error) { return nil, nil }
func (m *mockUserRepo) Update(ctx context.Context, u *entity.User) (*entity.User, error) { return u, nil }
func (m *mockUserRepo) Delete(ctx context.Context, id int64) error { return nil }

type mockCache struct{}
func (m *mockCache) Set(_ context.Context, _ string, _ any, _ time.Duration) error { return nil }
func (m *mockCache) Get(_ context.Context, _ string, _ any) error { return errors.New("miss") }
func (m *mockCache) Delete(_ context.Context, _ string) error { return nil }

type mockPublisher struct{}
func (m *mockPublisher) Publish(_ context.Context, _ string, _ any) error { return nil }

type mockNotifier struct{}
func (m *mockNotifier) SendEmail(_ context.Context, _, _, _ string) error { return nil }
func (m *mockNotifier) SendSMS(_ context.Context, _, _ string) error { return nil }

// ── Tests ──────────────────────────────────────────────────────────

func TestRegisterUser(t *testing.T) {
    tests := []struct {
        name    string
        cmd     input.RegisterUserCommand
        setup   func(*mockUserRepo)
        wantErr error
    }{
        {
            name: "success",
            cmd:  input.RegisterUserCommand{Name: "Budi", Email: "budi@test.com", Password: "pass123"},
            setup: func(r *mockUserRepo) {
                r.On("FindByEmail", mock.Anything, "budi@test.com").Return(nil, domain.ErrNotFound)
                r.On("Save", mock.Anything, mock.AnythingOfType("*entity.User")).
                    Return(&entity.User{ID: 1, Name: "Budi", Email: "budi@test.com"}, nil)
            },
            wantErr: nil,
        },
        {
            name: "email already exists",
            cmd:  input.RegisterUserCommand{Name: "Budi", Email: "budi@test.com", Password: "pass123"},
            setup: func(r *mockUserRepo) {
                r.On("FindByEmail", mock.Anything, "budi@test.com").Return(&entity.User{ID: 1}, nil)
            },
            wantErr: domain.ErrAlreadyExists,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            repo := new(mockUserRepo)
            tt.setup(repo)

            uc := usecase.NewUserUseCase(repo, &mockCache{}, &mockPublisher{}, &mockNotifier{})
            _, err := uc.Register(context.Background(), tt.cmd)

            assert.ErrorIs(t, err, tt.wantErr)
            repo.AssertExpectations(t)
        })
    }
}
```

---

## 13. Aturan Wajib

### Import Rules

| Package | ✅ Boleh Import | ❌ DILARANG Import |
|---|---|---|
| `domain/entity/` | stdlib only | Semua package internal lainnya |
| `usecase/port/` | `domain/`, stdlib | `delivery/`, `persistence/`, `infrastructure/`, framework apapun |
| `usecase/` | `domain/`, `usecase/port/`, stdlib | `delivery/`, `persistence/`, DB driver, HTTP lib, gRPC lib |
| `delivery/*/` | `usecase/port/input/`, `domain/` (errors), transport lib | `persistence/`, `infrastructure/database/` |
| `persistence/` | `domain/`, `usecase/port/output/`, DB/cache driver | `delivery/`, implementasi usecase |
| `infrastructure/lifecycle/` | stdlib only | `delivery/`, `usecase/`, `domain/` |
| `infrastructure/server/` | `delivery/`, `usecase/port/input/`, `lifecycle/`, server lib | `persistence/`, `bootstrap/` |
| `bootstrap/shared.go` | `config/`, `log/slog`, stdlib | DB driver, persistence, delivery |
| `bootstrap/apiserver.go` | `shared`, persistence, usecase, infra/server, lifecycle | — |
| `bootstrap/ouchclient.go` | `shared`, usecase, infra/server, lifecycle | persistence layer yang tidak relevan |
| `bootstrap/worker.go` | `shared`, persistence, usecase, lifecycle | delivery/http, delivery/grpc |
| `cmd/*/main.go` | `bootstrap.*App`, stdlib (`os`, `os/signal`, `syscall`) | Semua internal langsung |

### Coding Rules

```
1. Constructor Injection       Selalu New*(), tidak ada global state, tidak ada init() magic
2. Interface di sisi konsumen  Interface didefinisikan di usecase/port/, bukan di persistence/
3. Context selalu diteruskan   func Foo(ctx context.Context, ...) — wajib untuk semua I/O
4. Command ≠ Entity            Delivery layer membuat Command object, bukan langsung membuat entity
5. Entity ≠ Transport DTO      Jangan pernah kembalikan entity langsung ke transport layer
6. Domain errors di domain/    Sentinel errors di domain/errors.go, terjemahkan di setiap delivery
7. SQL hanya di persistence/   Tidak ada raw query di usecase atau delivery layer
8. Semua transport = Runnable  HTTP, gRPC, TCP, MQ consumer — semua implements lifecycle.Runnable
9. Satu bootstrap per binary   apiserver.go, ouchclient.go, worker.go — wiring berbeda, terpisah
10. shared.go hanya logger+cfg  DB, cache, MQ — instantiasi di masing-masing bootstrap, bukan shared
11. main.go seminimal mungkin  Hanya: signal context + bootstrap.New*App(ctx) + app.Start()
```

---

## 14. Checklist Kepatuhan

### Domain Layer
- [ ] Entity hanya berisi field domain dan domain methods
- [ ] Tidak ada import framework, DB driver, atau transport library
- [ ] Sentinel errors ada di `domain/errors.go`
- [ ] Value objects immutable

### Use Case Layer
- [ ] Hanya import `domain/` dan `usecase/port/`
- [ ] Tidak ada SQL, JSON encoding, HTTP logic, gRPC logic
- [ ] Semua dependency masuk via constructor parameter
- [ ] Input adalah Command object, output adalah Entity
- [ ] Context diteruskan ke semua method yang melakukan I/O

### Delivery Layer (semua transport)
- [ ] Hanya memanggil input port (use case interface)
- [ ] Mengkonversi format transport → Command (bukan entity)
- [ ] Mengkonversi Entity → format transport response
- [ ] Menerjemahkan domain errors ke kode error transport masing-masing
- [ ] Tidak ada business logic di handler
- [ ] Tidak ada direct import ke persistence layer

### Persistence Layer
- [ ] Mengimplementasikan output port (repository/cache interface)
- [ ] Semua query SQL/Redis command hanya ada di sini
- [ ] Mengembalikan domain errors jika data tidak ditemukan

### Bootstrap & Wiring
- [ ] `bootstrap/shared.go` hanya berisi logger & config — tidak ada DB, cache, atau MQ
- [ ] Setiap binary memiliki satu file bootstrap sendiri (`apiserver.go`, `ouchclient.go`, `worker.go`)
- [ ] Setiap bootstrap hanya menginstansiasi resource yang memang dibutuhkan binary tersebut
- [ ] `lifecycle.Runnable` diimplementasikan oleh setiap transport (HTTP, gRPC, TCP, MQ consumer)
- [ ] `lifecycle.Runner` yang mengatur concurrency dan error aggregation
- [ ] `Cleanup()` di setiap bootstrap menutup resource yang dibuka di bootstrap tersebut
- [ ] `cmd/*/main.go` hanya berisi: signal context + `bootstrap.New*App(ctx)` + `app.Start()`

### Tambah Binary Baru ✓
- [ ] Buat `cmd/<nama>/main.go` — hanya 10 baris (signal + bootstrap + start)
- [ ] Buat `bootstrap/<nama>.go` — struct App, `initialize()`, `Start()`, `Cleanup()`
- [ ] `initialize()` hanya instansiasi resource yang binary tersebut butuhkan
- [ ] Tidak ada perubahan di `usecase/` atau `domain/`
- [ ] Tidak ada perubahan di `bootstrap/shared.go`

---

## Referensi

- [The Clean Architecture — Uncle Bob (2012)](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html)
- [Hexagonal Architecture — Alistair Cockburn](https://alistair.cockburn.us/hexagonal-architecture/)
- [Go Proverbs — Rob Pike](https://go-proverbs.github.io/)

---

*Transport-agnostic Clean Architecture — bisnis logic tetap, transport datang dan pergi.*
