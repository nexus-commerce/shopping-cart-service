package main

import (
	"context"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	pb "github.com/nexus-commerce/nexus-contracts-go/cart/v1"
	product "github.com/nexus-commerce/nexus-contracts-go/product/v1"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"log"
	"net"
	"os"
	"os/signal"
	"shopping-cart-service/internal/cart"
	"shopping-cart-service/internal/config"
	"shopping-cart-service/internal/server"
	"strings"
	"syscall"
)

const (
	secret = "my-secret-key"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	// Config
	cfg, err := config.New()
	if err != nil {
		return err
	}

	// Redis client
	addr := fmt.Sprintf("%s:%s", cfg.DB.Host, cfg.DB.Port)
	rdb, err := newRedisClient(addr, cfg.DB.Password)
	if err != nil {
		return err
	}
	defer rdb.Close()

	// Listener
	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.Server.Port))
	if err != nil {
		return err
	}

	// gRPC client to Product Catalog Service
	productAddr := fmt.Sprintf("%s:%s", cfg.ProductClient.Host, cfg.ProductClient.Port)
	productClient, err := grpc.NewClient(productAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer productClient.Close()

	client := product.NewProductCatalogServiceClient(productClient)

	// Shopping cart instance
	shoppingCart := cart.New(rdb, 3600, client)

	// gRPC server with authentication interceptor
	s := grpc.NewServer(
		grpc.UnaryInterceptor(AuthInterceptor),
	)
	pb.RegisterShoppingCartServiceServer(s, &server.Server{
		Cart: shoppingCart,
	})

	// Serving gRPC server
	go func() {
		log.Printf("Starting gRPC shopping cart service server on port %s\n", cfg.Server.Port)
		if err = s.Serve(listener); err != nil {
			log.Println(err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit

	log.Println("Received shutdown signal, stopping server...")

	s.GracefulStop()

	return nil
}

func newRedisClient(addr, password string) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       0,
	})

	_, err := rdb.Ping(context.Background()).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %v", err)
	}

	return rdb, nil
}

func AuthInterceptor(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	mt, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "metadata is not provided")
	}

	bearedToken := mt["authorization"]
	fmt.Println(bearedToken)
	if len(bearedToken) == 0 {
		return nil, status.Error(codes.Unauthenticated, "bearer token not found")
	}

	tokenString := strings.TrimPrefix(bearedToken[0], "Bearer ")

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}

	claims := token.Claims.(jwt.MapClaims)

	userID, ok := claims["user-id"]
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "user-id not found")
	}

	userIDFloat, ok := userID.(float64)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "user-id is not float")
	}

	userIDInt := int(userIDFloat)

	ctx = context.WithValue(ctx, "user-id", userIDInt)

	return handler(ctx, req)
}
