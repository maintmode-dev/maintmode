package userpicker

import (
	"github.com/ruko1202/maintmode/internal/services/userpicker"
)

type Implementation struct {
	pickerSrv *userpicker.Service
}

func New(pickerSrv *userpicker.Service) *Implementation {
	return &Implementation{pickerSrv: pickerSrv}
}
