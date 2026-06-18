package modbus_receiver

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"net"
	"sync"
	"time"

	"noria-bearing-system/internal/database"
	"noria-bearing-system/internal/models"
	"noria-bearing-system/internal/modules/messages"
)

type ModbusReceiver struct {
	port        int
	bearingMap  map[uint16]int
	mu          sync.RWMutex
	listener    net.Listener
	running     bool
	onData      func(*models.SensorData)
	outputChan  chan<- messages.SensorDataMessage
}

type DataValidationResult struct {
	Valid   bool
	Message string
}

func NewModbusReceiver(port int, outputChan chan<- messages.SensorDataMessage) *ModbusReceiver {
	return &ModbusReceiver{
		port:       port,
		bearingMap: make(map[uint16]int),
		running:    false,
		outputChan: outputChan,
	}
}

func (s *ModbusReceiver) RegisterBearing(modbusAddr uint16, bearingID int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bearingMap[modbusAddr] = bearingID
}

func (s *ModbusReceiver) SetDataCallback(cb func(*models.SensorData)) {
	s.onData = cb
}

func (s *ModbusReceiver) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("Modbus服务器已在运行")
	}
	s.running = true
	s.mu.Unlock()

	addr := fmt.Sprintf(":%d", s.port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("Modbus TCP监听失败: %w", err)
	}
	s.listener = listener

	log.Printf("Modbus TCP 服务器启动在端口 %d", s.port)

	go func() {
		<-ctx.Done()
		s.Stop()
	}()

	go s.acceptLoop()
	return nil
}

func (s *ModbusReceiver) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}
	s.running = false
	if s.listener != nil {
		s.listener.Close()
	}
	close(s.outputChan)
	log.Println("Modbus TCP 服务器已停止")
}

func (s *ModbusReceiver) acceptLoop() {
	for {
		s.mu.RLock()
		running := s.running
		s.mu.RUnlock()
		if !running {
			break
		}

		conn, err := s.listener.Accept()
		if err != nil {
			s.mu.RLock()
			running := s.running
			s.mu.RUnlock()
			if running {
				log.Printf("Modbus 接受连接失败: %v", err)
			}
			continue
		}

		go s.handleConnection(conn)
	}
}

func (s *ModbusReceiver) handleConnection(conn net.Conn) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(30 * time.Second))

	buf := make([]byte, 260)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			return
		}
		if n < 8 {
			continue
		}

		resp := s.processMBAP(buf[:n])
		if resp != nil {
			conn.Write(resp)
		}
	}
}

func (s *ModbusReceiver) processMBAP(data []byte) []byte {
	if len(data) < 8 {
		return nil
	}

	transactionID := binary.BigEndian.Uint16(data[0:2])
	protocolID := binary.BigEndian.Uint16(data[2:4])
	length := binary.BigEndian.Uint16(data[4:6])
	unitID := data[6]

	_ = protocolID
	_ = length
	_ = unitID

	if len(data) < int(6+length) {
		return nil
	}

	pdu := data[7 : 6+length]
	functionCode := pdu[0]

	switch functionCode {
	case 0x03:
		return s.buildReadHoldingRegistersResponse(transactionID, unitID, pdu)
	case 0x06:
		return s.processWriteSingleRegister(transactionID, unitID, pdu)
	case 0x10:
		return s.processWriteMultipleRegisters(transactionID, unitID, pdu)
	default:
		return buildExceptionResponse(transactionID, unitID, functionCode, 0x01)
	}
}

func (s *ModbusReceiver) buildReadHoldingRegistersResponse(transactionID uint16, unitID byte, pdu []byte) []byte {
	if len(pdu) < 5 {
		return buildExceptionResponse(transactionID, unitID, pdu[0], 0x03)
	}
	startAddr := binary.BigEndian.Uint16(pdu[1:3])
	numRegs := binary.BigEndian.Uint16(pdu[3:5])

	respPDU := make([]byte, 2+numRegs*2)
	respPDU[0] = pdu[0]
	respPDU[1] = byte(numRegs * 2)

	for i := uint16(0); i < numRegs; i++ {
		val := uint16(0)
		addr := startAddr + i
		switch addr {
		case 0:
			val = uint16(time.Now().Unix() % 65536)
		case 1:
			val = 2500
		case 2:
			val = 45
		}
		binary.BigEndian.PutUint16(respPDU[2+i*2:], val)
	}

	return buildMBAP(transactionID, unitID, respPDU)
}

func (s *ModbusReceiver) processWriteSingleRegister(transactionID uint16, unitID byte, pdu []byte) []byte {
	if len(pdu) < 5 {
		return buildExceptionResponse(transactionID, unitID, pdu[0], 0x03)
	}

	regAddr := binary.BigEndian.Uint16(pdu[1:3])
	regValue := binary.BigEndian.Uint16(pdu[3:5])

	s.handleRegisterWrite(regAddr, float64(regValue))

	respPDU := make([]byte, 5)
	copy(respPDU, pdu[:5])
	return buildMBAP(transactionID, unitID, respPDU)
}

func (s *ModbusReceiver) processWriteMultipleRegisters(transactionID uint16, unitID byte, pdu []byte) []byte {
	if len(pdu) < 6 {
		return buildExceptionResponse(transactionID, unitID, pdu[0], 0x03)
	}

	startAddr := binary.BigEndian.Uint16(pdu[1:3])
	numRegs := binary.BigEndian.Uint16(pdu[3:5])
	byteCount := int(pdu[5])

	if len(pdu) < 6+byteCount {
		return buildExceptionResponse(transactionID, unitID, pdu[0], 0x03)
	}

	regData := pdu[6 : 6+byteCount]
	s.handleBearingRegisters(startAddr, numRegs, regData)

	respPDU := make([]byte, 5)
	respPDU[0] = pdu[0]
	binary.BigEndian.PutUint16(respPDU[1:], startAddr)
	binary.BigEndian.PutUint16(respPDU[3:], numRegs)
	return buildMBAP(transactionID, unitID, respPDU)
}

func (s *ModbusReceiver) handleRegisterWrite(addr uint16, value float64) {
	log.Printf("Modbus 单寄存器写入: addr=%d, value=%.2f", addr, value)
}

func (s *ModbusReceiver) validateSensorData(data *models.SensorData) DataValidationResult {
	if data.BearingID <= 0 {
		return DataValidationResult{Valid: false, Message: "无效的轴承ID"}
	}

	if data.Temperature < -40 || data.Temperature > 200 {
		return DataValidationResult{Valid: false, Message: fmt.Sprintf("温度值超出范围: %.2f°C", data.Temperature)}
	}

	if data.RadialLoad < 0 || data.RadialLoad > 1e6 {
		return DataValidationResult{Valid: false, Message: fmt.Sprintf("径向载荷超出范围: %.2fN", data.RadialLoad)}
	}

	if data.RotationalSpeed < 0 || data.RotationalSpeed > 10000 {
		return DataValidationResult{Valid: false, Message: fmt.Sprintf("转速超出范围: %.2fRPM", data.RotationalSpeed)}
	}

	if data.OilFilmThickness < 0 || data.OilFilmThickness > 100 {
		return DataValidationResult{Valid: false, Message: fmt.Sprintf("油膜厚度超出范围: %.4fμm", data.OilFilmThickness)}
	}

	return DataValidationResult{Valid: true, Message: ""}
}

func (s *ModbusReceiver) handleBearingRegisters(startAddr uint16, numRegs uint16, data []byte) {
	modbusAddressStep := uint16(10)
	s.mu.RLock()
	bearingID, ok := s.bearingMap[startAddr/modbusAddressStep]
	s.mu.RUnlock()

	if !ok {
		log.Printf("未找到Modbus地址 %d 对应的轴承", startAddr)
		s.sendToChannel(nil, false, fmt.Sprintf("未找到Modbus地址 %d 对应的轴承", startAddr))
		return
	}

	if int(numRegs)*2 < len(data) {
		numRegs = uint16(len(data) / 2)
	}

	floatRegs := make([]float64, numRegs)
	for i := uint16(0); i < numRegs && int(i*2+2) <= len(data); i++ {
		floatRegs[i] = parseFloat32FromRegs(data[i*2:])
	}

	sensorData := &models.SensorData{
		Time:             time.Now(),
		BearingID:        bearingID,
		Source:           "modbus",
	}

	if len(floatRegs) >= 1 {
		sensorData.Temperature = floatRegs[0]
	}
	if len(floatRegs) >= 2 {
		sensorData.RadialLoad = floatRegs[1]
	}
	if len(floatRegs) >= 3 {
		sensorData.RotationalSpeed = floatRegs[2]
	}
	if len(floatRegs) >= 4 {
		sensorData.OilFilmThickness = floatRegs[3]
	}

	validation := s.validateSensorData(sensorData)
	if !validation.Valid {
		log.Printf("传感器数据校验失败 (轴承 %d): %s", bearingID, validation.Message)
		s.sendToChannel(sensorData, false, validation.Message)
		return
	}

	log.Printf("收到轴承 %d 数据: 温度=%.2f°C, 载荷=%.2fN, 转速=%.2fRPM, 油膜=%.4fμm",
		bearingID, sensorData.Temperature, sensorData.RadialLoad,
		sensorData.RotationalSpeed, sensorData.OilFilmThickness)

	if database.Instance != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := database.Instance.InsertSensorData(ctx, sensorData); err != nil {
			log.Printf("保存传感器数据失败: %v", err)
		}
	}

	s.sendToChannel(sensorData, true, "")

	if s.onData != nil {
		s.onData(sensorData)
	}
}

func (s *ModbusReceiver) sendToChannel(data *models.SensorData, valid bool, errMsg string) {
	msg := messages.SensorDataMessage{
		Data:      data,
		Timestamp: time.Now(),
		Valid:     valid,
		Error:     errMsg,
	}

	select {
	case s.outputChan <- msg:
	default:
		log.Printf("Modbus输出通道已满，丢弃数据 (轴承ID=%v)", data)
	}
}

func parseFloat32FromRegs(data []byte) float64 {
	if len(data) < 4 {
		return 0
	}
	bits := binary.BigEndian.Uint32(data[0:4])
	return float64(math.Float32frombits(bits))
}

func buildMBAP(transactionID uint16, unitID byte, pdu []byte) []byte {
	length := uint16(1 + len(pdu))
	resp := make([]byte, 7+len(pdu))
	binary.BigEndian.PutUint16(resp[0:], transactionID)
	binary.BigEndian.PutUint16(resp[2:], 0)
	binary.BigEndian.PutUint16(resp[4:], length)
	resp[6] = unitID
	copy(resp[7:], pdu)
	return resp
}

func buildExceptionResponse(transactionID uint16, unitID byte, functionCode byte, exceptionCode byte) []byte {
	pdu := make([]byte, 2)
	pdu[0] = functionCode | 0x80
	pdu[1] = exceptionCode
	return buildMBAP(transactionID, unitID, pdu)
}
