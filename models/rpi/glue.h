#ifndef _GLUE_H_
#define _GLUE_H_

void setup_wiring_pi(void);
void setup_interrupts(void);

int digitalReadRx(void);
int digitalReadTx(void);
int digitalReadCE0(void);

#endif
