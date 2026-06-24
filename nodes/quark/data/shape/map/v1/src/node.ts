// quark/data/shape/map:v1
// Declaratively maps fields from the input payload to a new output shape.

export default {
    onMessage(message, publisher) {
        const mapping = config.get("mapping") || {};
        const preserve = config.getBoolean("preserve", false);
        const payload = message.getPayload();

        const result = {};

        // Apply mapping: source path -> target field
        for (const [sourcePath, targetField] of Object.entries(mapping)) {
            const value = getNestedValue(payload, sourcePath);
            if (value !== undefined) {
                setNestedValue(result, targetField, value);
            }
        }

        // Optionally preserve unmapped fields
        if (preserve) {
            for (const [key, value] of Object.entries(payload)) {
                if (!(key in result) && !Object.values(mapping).includes(key)) {
                    result[key] = value;
                }
            }
        }

        result._source = message.getSubject();
        publisher.publish("mapped", result);
    }
};

function getNestedValue(obj, path) {
    const parts = path.split(".");
    let current = obj;
    for (const part of parts) {
        if (current == null) return undefined;
        current = current[part];
    }
    return current;
}

function setNestedValue(obj, path, value) {
    const parts = path.split(".");
    let current = obj;
    for (let i = 0; i < parts.length - 1; i++) {
        if (!(parts[i] in current)) {
            current[parts[i]] = {};
        }
        current = current[parts[i]];
    }
    current[parts[parts.length - 1]] = value;
}
