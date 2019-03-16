package whatsapp

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"

	"github.com/dimaskiddo/go-whatsapp/binary"
	"github.com/dimaskiddo/go-whatsapp/crypto/cbc"
)

//writeJson enqueues a json message into the writeChan
func (wac *Conn) writeJson(data []interface{}) (<-chan string, error) {
	d, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	ts := time.Now().Unix()
	messageTag := fmt.Sprintf("%d.--%d", ts, wac.msgCount)
	bytes := []byte(fmt.Sprintf("%s,%s", messageTag, d))

	ch, err := wac.write(websocket.TextMessage, messageTag, bytes)
	if err != nil {
		return nil, err
	}

	wac.msgCount++
	return ch, nil
}

func (wac *Conn) writeBinary(node binary.Node, metric metric, flag flag, messageTag string) (<-chan string, error) {
	if len(messageTag) < 2 {
		return nil, ErrMissingMessageTag
	}

	data, err := wac.encryptBinaryMessage(node)
	if err != nil {
		return nil, errors.Wrap(err, "encryptBinaryMessage(node) failed")
	}

	bytes := []byte(messageTag + ",")
	bytes = append(bytes, byte(metric), byte(flag))
	bytes = append(bytes, data...)

	ch, err := wac.write(websocket.BinaryMessage, messageTag, bytes)
	if err != nil {
		return nil, errors.Wrap(err, "failed to write message")
	}

	wac.msgCount++
	return ch, nil
}

func (wac *Conn) sendKeepAlive() error {
	bytes := []byte("?,,")
	respChan, err := wac.write(websocket.TextMessage, "!", bytes)
	if err != nil {
		return errors.Wrap(err, "error sending keepAlive")
	}

	select {
	case resp := <-respChan:
		msecs, err := strconv.ParseInt(resp, 10, 64)
		if err != nil {
			return errors.Wrap(err, "error converting time string to uint")
		}
		wac.ServerLastSeen = time.Unix(msecs/1000, (msecs%1000)*int64(time.Millisecond))

	case <-time.After(wac.msgTimeout):
		return ErrConnectionTimeout
	}

	return nil
}

func (wac *Conn) write(messageType int, answerMessageTag string, data []byte) (<-chan string, error) {
	var ch chan string
	if answerMessageTag != "" {
		ch = make(chan string, 1)

		wac.listener.Lock()
		wac.listener.m[answerMessageTag] = ch
		wac.listener.Unlock()
	}

	wac.ws.Lock()
	err := wac.ws.conn.WriteMessage(messageType, data)
	wac.ws.Unlock()

	if err != nil {
		if answerMessageTag != "" {
			wac.listener.Lock()
			delete(wac.listener.m, answerMessageTag)
			wac.listener.Unlock()
		}
		return nil, errors.Wrap(err, "error writing to websocket")
	}
	return ch, nil
}

func (wac *Conn) encryptBinaryMessage(node binary.Node) (data []byte, err error) {
	b, err := binary.Marshal(node)
	if err != nil {
		return nil, errors.Wrap(err, "binary node marshal failed")
	}

	cipher, err := cbc.Encrypt(wac.session.EncKey, nil, b)
	if err != nil {
		return nil, errors.Wrap(err, "encrypt failed")
	}

	h := hmac.New(sha256.New, wac.session.MacKey)
	h.Write(cipher)
	hash := h.Sum(nil)

	data = append(data, hash[:32]...)
	data = append(data, cipher...)

	return data, nil
}
