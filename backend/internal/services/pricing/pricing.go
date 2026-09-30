package pricing

import "github.com/kelvins-io/12306cn/backend/internal/models"

// BasePrice returns adult price in fen for a seat segment.
func BasePrice(seatType, trainType string, fromSeq, toSeq int) int {
	segs := toSeq - fromSeq
	if segs < 1 {
		segs = 1
	}
	base := 5000
	switch trainType {
	case "G":
		base = 8000
	case "D":
		base = 6000
	case "K", "T":
		base = 3000
	}
	mult := 1.0
	switch seatType {
	case string(models.SeatFirst):
		mult = 1.6
	case string(models.SeatBusiness):
		mult = 3.0
	case string(models.SeatHardSleeper):
		mult = 1.4
	case string(models.SeatHard):
		mult = 0.8
	}
	return int(float64(base*segs) * mult)
}

// PassengerFactor returns discount by passenger type.
func PassengerFactor(passengerType string) float64 {
	switch passengerType {
	case "儿童":
		return 0.5
	case "学生":
		return 0.75
	case "军人":
		return 0.5
	default: // 成人
		return 1.0
	}
}

func PriceFor(seatType, trainType, passengerType string, fromSeq, toSeq int) int {
	base := BasePrice(seatType, trainType, fromSeq, toSeq)
	return int(float64(base) * PassengerFactor(passengerType))
}
