package store

import "errors"

// ErrLoginTaken is returned when registering a login that already exists.
var ErrLoginTaken = errors.New("login already taken")

// ErrInvalidCredentials is returned when login or password does not match.
var ErrInvalidCredentials = errors.New("invalid login or password")

// ErrOrderOwnedByOtherUser is returned when an order number is already linked to another account.
var ErrOrderOwnedByOtherUser = errors.New("order number belongs to another user")

// ErrInsufficientFunds is returned when a withdrawal exceeds the user's available balance.
var ErrInsufficientFunds = errors.New("insufficient loyalty balance")
