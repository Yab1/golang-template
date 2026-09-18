package mailer

import "context"

type Nop struct{}

func NewNop() *Nop {
	return &Nop{}
}

func (n *Nop) Send(context.Context, Message) error {
	return nil
}

func (n *Nop) Driver() string {
	return "nop"
}
