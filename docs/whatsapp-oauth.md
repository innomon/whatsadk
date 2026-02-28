# WhatsApp OAuth

A chat client, implemented as pure client side SPA (Single Page Application), deployed on web, wants to interact with a server an 
[ADK server, implemented in go](https://google.github.io/adk-docs/get-started/go/)

WhatsApp gateway (this project), is running behind the firewall, will be acting as the Auth provider.
The chat user interacts with this gateway via whatsapp chat. 

The chat client cannot directly communicate with the WhatsApp gateway.

Refer to [REVERSE OTP Implementation](WHATSADK-REVERSE_OTP_IMPLEMENTATION_PLAN.md), and create a flow,
which may be different than the current reverse OTP flow.

also refer [TOTP](totp.md)

Suggest a flow, where the chat client gets JWT key  to be sent to the ADK server.

Should we use :
1. JWT Ed25519 token.   
2. https://github.com/nats-io/nkeys , https://github.com/nats-io/jwt

