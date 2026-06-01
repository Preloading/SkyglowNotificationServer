# How 2 send a notification

Code & Libraries are available at https://github.com/Preloading/SkyglowNotificationLibraries. This is mainly intended if you want to write your own.

## Vocab
- SGN = **S**ky**g**low **N**otifications
- APNS = **A**pple **P**ush **N**otification **S**ervice

SGN is built as a fill-in for APNS, and attempts to be roughly compatible with APNS on the device side. However, sending notifications is different with SGN.

## Getting routing info
APNS and thus SGN is designed to use tokens in order to send a notification to the correct device. As we are trying to fit within some of the confines of APNS, we follow the restrictions it provides, including the tokens being 32 bytes long. The app can choose how it can transmit tokens, so if you do not control the app, you will have to reverse engineer it to find where it sends it off.

The token given from SGN fits into this format
```
|7072656c6f6164696e672e6465760000   adf9650cd3c04b5523a93fb847ea6645
|________________________________| |________________________________|
        server identifier                   "K", random bytes
```

Server identifier is a domain with a corrisponding _sgn.{DOMAIN} TXT record, while K is random bytes. Server identifier & K are both 16 bytes. This token should be always be a secret. The server identifer is padded with 0x00 bytes at the end if the domain is not exactly 16 bytes. 

To send off the server, you need to extract both the server identier & K. The routing key is retrieved by SHA256'ing the K value. For this key, it would be
```
SHA256(fromHexToBytes(adf9650cd3c04b5523a93fb847ea6645))
```
From the example token, we get a routing token of
```
ffe6d6ebf6b0acc38bfc0793ac8989b95c39238586cb645d1b4df9253ba6ac4b
```

## Sending a notification
With the routing info obtained, you can start sending off a notifiction. You must first pick a skyglow notification server trusted by your app. For this example, we will use `preloading.dev`. You must first obtain it's corrisponding HTTP service. To obtain it, you must query the TXT record of _sgn.{DOMAIN}. In our example, it would be `_sgn.preloading.dev`. Querying this, we get:
```
"tcp_addr=69.69.42.02 tcp_port=21138 http_addr=https://sgnprod.preloading.dev"
```
This contains server bits of information used for both the device, server, etc. The only thing you need to parse is the HTTP URL. It is space seperated, with an "equals" system switching from key to value. It may be wise to only allow HTTPS aswell.

With this, you should send a POST request to that server, as `{http_addr}/send`. In our example, it would be `https://sgnprod.preloading.dev/send`. The body should look like:
```json
{
	"data": {
        "aps": {
            "alert": "you just lost the game! click here to claim your prize!",
            "sound": "default"
        }
	},
	"routing_key": "ffe6d6ebf6b0acc38bfc0793ac8989b95c39238586cb645d1b4df9253ba6ac4b",
	"server_address": "preloading.dev"
}
```

The `routing_key` and `server_address` should be from the getting the routing info step. The contents of `data` is the exact same from APNS. You should check the [APNS docs](https://developer.apple.com/documentation/usernotifications/generating-a-remote-notification) for what to put in here. You will get a response like
```json
{
	"data": {
		"data": {
			"aps": {
				"alert": "you just lost the game! click here to claim your prize!",
				"sound": "default"
			}
		},
		"ciphertext": null,
		"data_type": "",
		"iv": null,
		"routing_key": "ffe6d6ebf6b0acc38bfc0793ac8989b95c39238586cb645d1b4df9253ba6ac4b",
		"server_address": "preloading.dev",
		"topic": ""
	},
	"status": "success"
}
```
And now you just sent a notification!

There is no guarentee that a notification will actually be delivered, even if it says "success". If the device is offline, this server should cache one notification per token, but it is not guarenteed. It can also be lost, or sent into a fire. 

## Sending encrypted notifications
If you are a security nerd, or don't want your notification being read by the potentially two servers in the middle, you can encrypt the notification. 

You should format what you previously had in the `data` on the unencrypted one as either JSON or PList. Obtaining the key to encrypt should look like:
```
e2ee_key = HKDF-SHA256( // this can be cached!
    key_material  = K,
    salt          = UTF8(server_address) + "Hello from the Skyglow Notifications developers!",
    info          = <empty>,
    output_length = 32
)
```

Then, you can encrypt your obtained payload like:
```
iv = SecureRandom(12) // random 12 bytes, used as a nonce. this should be generated using a secure random system.

ciphertext, tag = AES-256-GCM-Encrypt(
    key       = e2ee_key,
    iv        = iv,
    plaintext = payload,
    aad       = <none>
)
```
After obtaining your token, you should send off the data through the same endpoint used in sending an unencrypted payload. The body however, should look like this:
```json
{
	"ciphertext": "SSB3YXMgdG9vIGxhenkgdG8gYWN0dWFsbHkgZ28gYW5kIGF0dGVtcHQgdG8gbWFrZSBhIG5vdGlmaWNhdGlvbiB0aGF0IHdhcyBlbmNyeXB0ZWQsIHNvIGhlcmUncyBzb21lIHJhbmRvbSB0ZXh0IHRvIGZpbGwgaW4gdGhhdCBnYXAuIFRoZSBkYXRhIHNob3VsZCBub3QgbG9vayBsaWtlIHRoaXMsIG9idmlvdXNseS4=.....",
    "iv":"JMF1lH1AbTTfV5Cy",
    "data_type":"json", // can also be `plist` if you encoded it with that.
    "is_encrypted":true,

	"routing_key": "ffe6d6ebf6b0acc38bfc0793ac8989b95c39238586cb645d1b4df9253ba6ac4b",
	"server_address": "preloading.dev"
}
```
If everything goes well, you should have just sent a notification that no one, execpt you and the device, knows about!


NOTES:
- This **does not include forward secrecy!** If your token gets leaked, **anyone will be able to read past and future contents**. So uhh don't leak them x3

## Feedback
Feedback can be issued by the server, which contains data such as removed tokens. To get this data, you must create a 256 (you can probably change this depending on the server) byte token used to register and fetch this data.

# TODO: finish this