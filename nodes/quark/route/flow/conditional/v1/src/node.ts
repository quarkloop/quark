// quark/route/flow/conditional:v1
// Routes messages to different events based on content predicates.
// First matching rule wins. If no rule matches, the message is dropped.

export default {
    onMessage(message, publisher) {
        const rules = config.get("rules") || [];
        const payload = message.getPayload();

        for (const rule of rules) {
            if (matchRule(rule.when, payload)) {
                publisher.publish(rule.emit, payload);
                return;
            }
        }

        // No rule matched — message is dropped
    }
};

function matchRule(expr, payload) {
    if (!expr) return true;

    // Support simple expressions like:
    //   "payload.level === 'error'"
    //   "payload.value > 100"
    //   "payload.status === 'active'"
    try {
        const fn = new Function("payload", `"use strict"; return (${expr});`);
        return fn(payload) === true;
    } catch (e) {
        return false;
    }
}
