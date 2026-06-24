// quark/net/http/fetch:v1
// Fetches a URL from the message payload and emits the response body.

export default {
    onMessage(message, publisher) {
        const urlField = config.getString("urlField", "url");
        const method = config.getString("method", "GET");
        const headers = config.get("headers") || {};
        const timeout = config.getInt("timeout", 30000);
        const payload = message.getPayload();

        const url = payload[urlField] || payload["url"];
        if (!url) {
            publisher.publish("error", {
                error: "No URL found in payload field: " + urlField,
                source: message.getSubject()
            });
            return;
        }

        try {
            const response = fetchUrl(url, method, headers, timeout);
            publisher.publish("response", {
                url: url,
                status: response.status,
                body: response.body,
                source: message.getSubject()
            });
        } catch (e) {
            publisher.publish("error", {
                error: e.message,
                url: url,
                source: message.getSubject()
            });
        }
    }
};

function fetchUrl(url, method, headers, timeoutMs) {
    // Use Java interop for HTTP — GraalJS doesn't have native fetch
    // The JsPublisher bridge provides host access, so we use Java's HttpClient
    var HttpClient = Java.type("java.net.http.HttpClient");
    var HttpRequest = Java.type("java.net.http.HttpRequest");
    var HttpResponse = Java.type("java.net.http.HttpBodyHandlers");

    var client = HttpClient.newBuilder()
        .connectTimeout(java.time.Duration.ofMillis(timeoutMs))
        .build();

    var requestBuilder = HttpRequest.newBuilder()
        .uri(new java.net.URI(url))
        .timeout(java.time.Duration.ofMillis(timeoutMs))
        .method(method);

    for (var key in headers) {
        requestBuilder.header(key, headers[key]);
    }

    var request = requestBuilder.build();
    var response = client.send(request, HttpResponse.ofString());

    return {
        status: response.statusCode(),
        body: response.body()
    };
}
