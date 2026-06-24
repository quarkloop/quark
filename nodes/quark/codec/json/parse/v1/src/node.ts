// quark/codec/json/parse:v1
// Parses a JSON string from the message payload into an object.

export default {
    onMessage(message, publisher) {
        const field = config.getString("field", "data");
        const strict = config.getBoolean("strict", false);
        const payload = message.getPayload();
        const raw = payload[field] || payload["data"] || JSON.stringify(payload);

        try {
            const parsed = typeof raw === "string" ? JSON.parse(raw) : raw;
            publisher.publish("parsed", { data: parsed, source: message.getSubject() });
        } catch (e) {
            if (strict) {
                throw e;
            }
            publisher.publish("error", {
                error: e.message,
                input: raw,
                source: message.getSubject()
            });
        }
    }
};
