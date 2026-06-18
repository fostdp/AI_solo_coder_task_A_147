package modbus

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
)

type Server struct {
	port        int
	bearingMap  map[uint16]int
	mu          sync.RWMutex
	listener    net.Listener
	running     bool
	onData      func(*models.SensorData)
}

func NewServer(port int) *Server {
	return &Server{
		port:       port,
		bearingMap: make(map[uint16]int),
		running:    false,
	}
}

func (s *Server) RegisterBearing(modbusAddr uint16, bearingID int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bearingMap[modbusAddr] = bearingID
}

func (s *Server) SetDataCallback(cb func(*models.SensorData)) {
	s.onData = cb
}

func (s *Server) Start(ctx context.Context) error {
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

func (s *Server) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}
	s.running = false
	if s.listener != nil {
		s.listener.Close()
	}
	log.Println("Modbus TCP 服务器已停止")
}

func (s *Server) acceptLoop() {
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

func (s *Server) handleConnection(conn net.Conn) {
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

func (s *Server) processMBAP(data []byte) []byte {
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

func (s *Server) buildReadHoldingRegistersResponse(transactionID uint16, unitID byte, pdu []byte) []byte {
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

func (s *Server) processWriteSingleRegister(transactionID uint16, unitID byte, pdu []byte) []byte {
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

func (s *Server) processWriteMultipleRegisters(transactionID uint16, unitID byte, pdu []byte) []byte {
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

func (s *Server) handleRegisterWrite(addr uint16, value float64) {
	log.Printf("Modbus 单寄存器写入: addr=%d, value=%.2f", addr, value)
}

func (s *Server) handleBearingRegisters(startAddr uint16, numRegs uint16, data []byte) {
	s.mu.RLock()
	bearingID, ok := s.bearingMap[startAddr/10]
	s.mu.RUnlock()

	if !ok {
		log.Printf("未找到Modbus地址 %d 对应的轴承", startAddr)
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

	if s.onData != nil {
		s.onData(sensorData)
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
