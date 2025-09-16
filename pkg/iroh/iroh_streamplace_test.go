package iroh_streamplace

import (
	"testing"

	"github.com/stretchr/testify/assert"

	iroh "stream.place/streamplace/pkg/iroh/generated/iroh_streamplace"
)

type Message struct {
	topic string
	data  []byte
}

type TestHandler struct {
	messages chan Message
}

func (handler TestHandler) HandleData(topic string, data []byte) {
	handler.messages <- Message{topic, data}
}

func TestBasicRoundtrip(t *testing.T) {
	ep1, err := iroh.NewEndpoint()
	assert.Nil(t, err)
	sender, err := iroh.NewSender(ep1)
	assert.NoError(t, err.AsError())

	messages := make(chan Message, 5)
	handler := TestHandler{messages: messages}

	ep2, err := iroh.NewEndpoint()
	assert.NoError(t, err.AsError())
	receiver, err := iroh.NewReceiver(ep2, &handler)
	assert.NoError(t, err.AsError())

	senderAddr := sender.NodeAddr()
	senderId := senderAddr.NodeId()

	// subscribe
	err = receiver.Subscribe(senderId, "foo")
	assert.NoError(t, err.AsError())

	// send a few messages
	for i := range 5 {
		err = sender.Send("foo", []byte{byte(i), 0, 0, 0})
		assert.NoError(t, err.AsError())
	}

	// make sure the receiver got them
	for i := range 5 {
		msg := <-messages
		assert.Equal(t, msg.topic, "foo")
		assert.Equal(t, msg.data, []byte{byte(i), 0, 0, 0})
	}
}
