#include "glue.h"
#include <wiringPi.h>
#include "_cgo_export.h"

// CAN_RX is on BCM_GPIO_25 aka WiringPin 6
// CAN_TX is on BCM_GPIO_22 aka WiringPin 3
// MCU_RESET is on BCM_GPIO_26 aka WiringPin 25

#define CAN_RX_PIN 6
#define CAN_TX_PIN 3
#define MCU_RESET_PIN 25

int digitalReadRx(void)
{
    return digitalRead(CAN_RX_PIN);
}

int digitalReadTx(void)
{
    return digitalRead(CAN_TX_PIN);
}

void setup_wiring_pi(void)
{
    wiringPiSetup();
    pinMode(CAN_RX_PIN, INPUT);
    pinMode(CAN_TX_PIN, INPUT);
    pinMode(MCU_RESET_PIN, INPUT); // avoid leaving the MCU stuck at reset
    pullUpDnControl(CAN_TX_PIN, PUD_DOWN);
}

void setup_interrupts()
{
    wiringPiISR(CAN_RX_PIN, INT_EDGE_FALLING, CanRxInterrupt);
    //wiringPiISR(CAN_TX_PIN, INT_EDGE_FALLING, CanTxInterrupt);
}
