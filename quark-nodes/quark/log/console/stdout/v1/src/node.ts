// quark/log/console/stdout:v1
// Writes each incoming message payload to standard output as JSON.

export default {
    onMessage: function(message, publisher) {
        const payload = message.getPayload();
        console.log(JSON.stringify(payload));
    }
};
