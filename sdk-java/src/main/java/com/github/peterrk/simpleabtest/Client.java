package com.github.peterrk.simpleabtest;

import java.io.ByteArrayInputStream;
import java.io.IOException;
import java.io.InputStream;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.time.Instant;
import java.util.List;
import java.util.Map;
import java.util.zip.GZIPInputStream;
import okhttp3.OkHttpClient;
import okhttp3.Request;
import okhttp3.Response;
import okhttp3.ResponseBody;

public final class Client implements AutoCloseable {
    private final String srcUrl;
    private final String token;
    private final OkHttpClient httpClient;
    private final long ttlSeconds;
    private volatile List<Model.Experiment> data;
    private volatile long stamp;
    private volatile boolean active;
    private Thread refreshThread;

    private Client(String srcUrl, String token, long ttlSeconds, OkHttpClient httpClient) {
        this.srcUrl = srcUrl;
        this.token = token;
        this.ttlSeconds = ttlSeconds;
        this.httpClient = httpClient;
    }

    public static Client create(String address, long appid, String accessToken, long ttlSeconds)
            throws IOException, InterruptedException {
        return create(address, appid, accessToken, ttlSeconds,
                new OkHttpClient.Builder()
                        .connectTimeout(Duration.ofSeconds(10))
                        .readTimeout(Duration.ofSeconds(10))
                        .build());
    }

    static Client create(String address, long appid, String accessToken, long ttlSeconds, OkHttpClient httpClient)
            throws IOException, InterruptedException {
        long refreshTtl = ttlSeconds;
        if (refreshTtl > 0 && refreshTtl <= 60) {
            refreshTtl = 60;
        }

        Client client = new Client(joinUrl(address, appid), accessToken, refreshTtl, httpClient);
        client.refresh();
        client.startRefreshLoop();
        return client;
    }

    public synchronized void refresh() throws IOException {
        Request request = new Request.Builder()
                .url(srcUrl)
                .header("ACCESS_TOKEN", token)
                .header("Accept-Encoding", "gzip")
                .build();

        try (Response response = httpClient.newCall(request).execute()) {
            if (!response.isSuccessful()) {
                throw new IOException("fetch app info failed: status=" + response.code());
            }
            data = Model.parseExperiments(decodeBody(response));
        }
        stamp = Instant.now().getEpochSecond();
    }

    public long stamp() {
        return stamp;
    }

    public Decision ab(String key, Map<String, String> ctx) {
        List<Model.Experiment> snapshot = data;
        if (snapshot == null) {
            return new Decision(Map.of(), List.of());
        }
        return Model.getExpConfig(snapshot, key, ctx);
    }

    @Override
    public void close() {
        active = false;
        Thread thread = refreshThread;
        if (thread != null) {
            thread.interrupt();
            try {
                thread.join();
            } catch (InterruptedException ex) {
                Thread.currentThread().interrupt();
            }
        }
    }

    static List<Model.Experiment> parseExperiments(String payload) {
        return Model.parseExperiments(payload);
    }

    private void startRefreshLoop() {
        if (ttlSeconds == 0) {
            return;
        }
        active = true;
        refreshThread = Thread.ofPlatform().daemon().name("simple-abtest-refresh").start(() -> {
            while (active) {
                try {
                    Thread.sleep(ttlSeconds * 1000);
                    refresh();
                } catch (InterruptedException ex) {
                    Thread.currentThread().interrupt();
                    break;
                } catch (IOException ignored) {
                }
            }
        });
    }

    private static String joinUrl(String address, long appid) {
        String base = address.endsWith("/") ? address.substring(0, address.length() - 1) : address;
        return base + "/app/" + appid;
    }

    private static String decodeBody(Response response) throws IOException {
        ResponseBody body = response.body();
        if (body == null) {
            throw new IOException("fetch app info failed: empty body");
        }
        byte[] bytes = body.bytes();
        String encoding = response.header("Content-Encoding", "");
        if ("gzip".equalsIgnoreCase(encoding)) {
            try (InputStream raw = new ByteArrayInputStream(bytes);
                 GZIPInputStream gzip = new GZIPInputStream(raw)) {
                return new String(gzip.readAllBytes(), StandardCharsets.UTF_8);
            }
        }
        return new String(bytes, StandardCharsets.UTF_8);
    }
}
