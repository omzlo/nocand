package nocan

type MessageType byte

const (
	PUBLISH                          MessageType = iota
	SYS_ADDRESS_REQUEST                          //= 1
	SYS_ADDRESS_CONFIGURE                        //= 2
	SYS_ADDRESS_CONFIGURE_ACK                    //= 3
	SYS_ADDRESS_LOOKUP                           //= 4
	SYS_ADDRESS_LOOKUP_ACK                       //= 5
	SYS_NODE_BOOT_REQUEST                        //= 6
	SYS_NODE_BOOT_ACK                            //= 7
	SYS_NODE_PING                                //= 8
	SYS_NODE_PING_ACK                            //= 9
	SYS_CHANNEL_REGISTER                         //= 10
	SYS_CHANNEL_REGISTER_ACK                     //= 11
	SYS_CHANNEL_UNREGISTER                       //= 12
	SYS_CHANNEL_UNREGISTER_ACK                   //= 13
	SYS_CHANNEL_SUBSCRIBE                        //= 14
	SYS_CHANNEL_UNSUBSCRIBE                      //= 15
	SYS_CHANNEL_LOOKUP                           //= 16
	SYS_CHANNEL_LOOKUP_ACK                       //= 17
	SYS_BOOTLOADER_GET_SIGNATURE                 //= 18
	SYS_BOOTLOADER_GET_SIGNATURE_ACK             //= 19
	SYS_BOOTLOADER_SET_ADDRESS                   //= 20
	SYS_BOOTLOADER_SET_ADDRESS_ACK               //= 21
	SYS_BOOTLOADER_WRITE                         //= 22
	SYS_BOOTLOADER_WRITE_ACK                     //= 23
	SYS_BOOTLOADER_READ                          //= 24
	SYS_BOOTLOADER_READ_ACK                      //= 25
	SYS_BOOTLOADER_LEAVE                         //= 26
	SYS_BOOTLOADER_LEAVE_ACK                     //= 27
	SYS_BOOTLOADER_ERASE                         //= 28
	SYS_BOOTLOADER_ERASE_ACK                     //= 29
	SYS_RESERVED                                 //= 30
	SYS_DEBUG_MESSAGE                            //= 31
	MESSAGE_TYPE_COUNT
)

var nocan_message_type_strings = [MESSAGE_TYPE_COUNT]string{
	"nocan-publish",
	"nocan-sys-address-request",
	"nocan-sys-address-configure",
	"nocan-sys-address-configure-ack",
	"nocan-sys-address-lookup",
	"nocan-sys-address-lookup-ack",
	"nocan-sys-node-boot-request",
	"nocan-sys-node-boot-ack",
	"nocan-sys-node-ping",
	"nocan-sys-node-ping-ack",
	"nocan-sys-channel-register",
	"nocan-sys-channel-register-ack",
	"nocan-sys-channel-unregister",
	"nocan-sys-channel-unregister-ack",
	"nocan-sys-channel-subscribe",
	"nocan-sys-channel-unsubscribe",
	"nocan-sys-channel-lookup",
	"nocan-sys-channel-lookup-ack",
	"nocan-sys-bootloader-get-signature",
	"nocan-sys-bootloader-get-signature-ack",
	"nocan-sys-bootloader-set-address",
	"nocan-sys-bootloader-set-address-ack",
	"nocan-sys-bootloader-write",
	"nocan-sys-bootloader-write-ack",
	"nocan-sys-bootloader-read",
	"nocan-sys-bootloader-read-ack",
	"nocan-sys-bootloader-leave",
	"nocan-sys-bootloader-leave-ack",
	"nocan-sys-bootloader-erase",
	"nocan-sys-bootloader-erase-ack",
	"nocan-sys-reserved",
	"nocan-sys-debug-message",
}

func (mt MessageType) String() string {
	if mt >= MessageType(len(nocan_message_type_strings)) {
		return "nocan-unknown"
	}
	return nocan_message_type_strings[mt]
}
