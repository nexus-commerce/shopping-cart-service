package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	pb "github.com/nexus-commerce/nexus-contracts-go/cart/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log"
	"shopping-cart-service/internal/cart"
)

type Server struct {
	pb.UnimplementedShoppingCartServiceServer
	Cart *cart.ShoppingCart
}

func (s *Server) GetCart(ctx context.Context, _ *pb.GetCartRequest) (*pb.GetCartResponse, error) {
	userID, ok := ctx.Value("user-id").(int)
	if !ok {
		return nil, status.Error(codes.Internal, "user id missing")
	}

	userIDInt := int64(userID)

	resItems, totalPrice, totalItems, err := s.Cart.GetCart(ctx, userIDInt)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	items := make([]*pb.CartItem, 0, len(resItems))
	for sku, details := range resItems {
		var jsonMap cart.Item
		err = json.Unmarshal([]byte(details), &jsonMap)
		if err != nil {
			return nil, status.Error(codes.Internal, fmt.Sprintf("failed to unmarshal cart item: %v", err))
		}

		items = append(items, &pb.CartItem{
			Quantity:       jsonMap.Quantity,
			Price:          jsonMap.Price,
			Sku:            sku,
			Name:           jsonMap.Name,
			ImageUrl:       jsonMap.ImageURL,
			ItemTotalPrice: jsonMap.ItemTotalPrice,
		})
	}

	return &pb.GetCartResponse{
		Items:      items,
		TotalPrice: totalPrice,
		TotalItems: totalItems,
	}, nil
}

func (s *Server) AddItem(ctx context.Context, r *pb.AddItemRequest) (*pb.AddItemResponse, error) {
	userID, ok := ctx.Value("user-id").(int)
	if !ok {
		return nil, status.Error(codes.Internal, "user id missing") // return FAILED_PRECONDITION status here as the system should never get into this state
	}

	userIDInt := int64(userID)

	item, err := s.Cart.AddItem(ctx, userIDInt, r.GetQuantity(), r.GetSku())
	if err != nil {
		log.Println(err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.AddItemResponse{
		Item: &pb.CartItem{
			Quantity:       item.Quantity,
			Price:          item.Price,
			Sku:            r.GetSku(),
			Name:           item.Name,
			ImageUrl:       item.ImageURL,
			ItemTotalPrice: item.ItemTotalPrice,
		},
	}, nil
}

func (s *Server) UpdateItem(ctx context.Context, r *pb.UpdateItemRequest) (*pb.UpdateItemResponse, error) {
	userID, ok := ctx.Value("user-id").(int)
	if !ok {
		return nil, status.Error(codes.Internal, "user id missing") // return FAILED_PRECONDITION status here as the system should never get into this state
	}

	userIDInt := int64(userID)

	item, err := s.Cart.UpdateItemQuantity(ctx, userIDInt, r.GetQuantity(), r.GetSku())
	if err != nil {
		if errors.Is(err, cart.ErrItemNotFound) {
			return nil, status.Error(codes.NotFound, "item not found in cart")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.UpdateItemResponse{
		Item: &pb.CartItem{
			Quantity:       item.Quantity,
			Price:          item.Price,
			Sku:            r.GetSku(),
			Name:           item.Name,
			ImageUrl:       item.ImageURL,
			ItemTotalPrice: item.ItemTotalPrice,
		},
	}, nil
}

func (s *Server) RemoveItem(ctx context.Context, r *pb.RemoveItemRequest) (*pb.RemoveItemResponse, error) {
	userID, ok := ctx.Value("user-id").(int)
	if !ok {
		return nil, status.Error(codes.Internal, "user id missing")
	}

	userIDInt := int64(userID)

	s.Cart.RemoveItem(ctx, userIDInt, r.GetSku())

	return &pb.RemoveItemResponse{}, nil
}

func (s *Server) ClearCart(ctx context.Context, _ *pb.ClearCartRequest) (*pb.ClearCartResponse, error) {
	userID, ok := ctx.Value("user-id").(int)
	if !ok {
		return nil, status.Error(codes.Internal, "user id missing")
	}

	userIDInt := int64(userID)

	s.Cart.ClearCart(ctx, userIDInt)

	return &pb.ClearCartResponse{}, nil
}
