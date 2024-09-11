package main

import "fmt"

type actionType int

const (
	create actionType = iota
	delete
)

func (a actionType) String() string {
	return [...]string{"create", "delete"}[a]
}

func validateActionType(action string) (actionType, error) {
	switch action {
	case "create":
		return create, nil
	case "delete":
		return delete, nil
	default:
		return -1, fmt.Errorf("invalid action type: %s", action)
	}
}
