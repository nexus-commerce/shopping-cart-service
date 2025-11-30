package cart

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	product "github.com/nexus-commerce/nexus-contracts-go/product/v1"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"time"
)

var (
	ErrItemNotFound      = errors.New("item not found in cart")
	ErrProductNotFound   = errors.New("product not found")
	ErrInsufficientStock = errors.New("insufficient stock for product")
	ErrInvalidQuantity   = errors.New("invalid quantity")
	ErrInvalidSKU        = errors.New("invalid SKU")
)

type ShoppingCart struct {
	redisClient   *redis.Client
	productClient product.ProductCatalogServiceClient
	ttl           int64
}

type Item struct {
	Quantity       int32
	Price          float64
	Name           string
	ImageURL       string
	ItemTotalPrice float64
}

func New(redisClient *redis.Client, ttl int64, productClient product.ProductCatalogServiceClient) *ShoppingCart {
	return &ShoppingCart{
		redisClient:   redisClient,
		ttl:           ttl,
		productClient: productClient,
	}
}

func (c *ShoppingCart) GetCart(ctx context.Context, userID int64) (map[string]string, float64, int32, error) {
	key := fmt.Sprintf("cart:%d", userID)
	res, err := c.redisClient.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, 0.0, 0, err
	}

	var totalPrice float64
	var totalItems int32
	for _, details := range res {
		var CartItem Item
		err = json.Unmarshal([]byte(details), &CartItem)
		if err != nil {
			return nil, 0.0, 0, err
		}

		totalPrice += CartItem.Price * float64(CartItem.Quantity)
		totalItems += CartItem.Quantity
	}

	return res, totalPrice, totalItems, err
}

func (c *ShoppingCart) AddItem(ctx context.Context, userID int64, quantity int32, sku string) (*Item, error) {
	if quantity <= 0 {
		return nil, ErrInvalidQuantity
	}

	if sku == "" {
		return nil, ErrInvalidSKU
	}

	key := fmt.Sprintf("cart:%d", userID)

	p, err := c.productClient.GetProductBySKU(ctx, &product.GetProductBySKURequest{Sku: sku})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, ErrProductNotFound
		}
		return nil, err
	}

	if p.GetProduct().GetStockQuantity() < quantity {
		return nil, ErrInsufficientStock
	}

	item := Item{
		Quantity:       quantity,
		Price:          p.Product.Price,
		Name:           p.Product.Name,
		ImageURL:       p.Product.ImageUrl,
		ItemTotalPrice: float64(quantity) * p.Product.Price,
	}

	encodedValue, err := json.Marshal(item)
	if err != nil {
		return nil, err
	}

	_, err = c.redisClient.HSet(ctx, key, sku, encodedValue).Result()
	if err != nil {
		return nil, err
	}

	_, err = c.redisClient.Expire(ctx, key, time.Duration(c.ttl)*time.Second).Result()
	if err != nil {
		return nil, err
	}

	return &item, nil
}

func (c *ShoppingCart) UpdateItemQuantity(ctx context.Context, userID int64, quantity int32, sku string) (*Item, error) {
	if quantity <= 0 {
		return nil, ErrInvalidQuantity
	}

	if sku == "" {
		return nil, ErrInvalidSKU
	}

	key := fmt.Sprintf("cart:%d", userID)

	result, err := c.redisClient.HGet(ctx, key, sku).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrItemNotFound
		}
		return nil, err
	}

	p, err := c.productClient.GetProductBySKU(ctx, &product.GetProductBySKURequest{Sku: sku})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, ErrProductNotFound
		}
		return nil, err
	}

	if p.GetProduct().GetStockQuantity() < quantity {
		return nil, ErrInsufficientStock
	}

	var item Item

	err = json.Unmarshal([]byte(result), &item)
	if err != nil {
		return nil, err
	}

	item.Quantity = quantity

	encodedValue, err := json.Marshal(item)
	if err != nil {
		return nil, err
	}

	_, err = c.redisClient.HSet(ctx, key, sku, encodedValue).Result()
	if err != nil {
		return nil, err
	}

	_, err = c.redisClient.Expire(ctx, key, time.Duration(c.ttl)*time.Second).Result()
	if err != nil {
		return nil, err
	}

	return &item, nil

}

func (c *ShoppingCart) RemoveItem(ctx context.Context, userID int64, sku string) {
	key := fmt.Sprintf("cart:%d", userID)
	c.redisClient.HDel(ctx, key, sku)
}

func (c *ShoppingCart) ClearCart(ctx context.Context, userID int64) {
	key := fmt.Sprintf("cart:%d", userID)
	c.redisClient.Del(ctx, key)
}
