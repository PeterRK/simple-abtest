package com.github.peterrk.simpleabtest;

import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.util.Base64;
import java.util.List;
import java.util.Map;
import java.util.zip.GZIPOutputStream;
import okhttp3.Interceptor;
import okhttp3.MediaType;
import okhttp3.OkHttpClient;
import okhttp3.Protocol;
import okhttp3.Request;
import okhttp3.Response;
import okhttp3.ResponseBody;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertArrayEquals;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertInstanceOf;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

class SdkJavaTest {
    private static final long[] HASH_EXPECTED = {
            0x232706fc6bf50919L, 0x50209687d54ec67eL, 0xfbe67d8368f3fb4fL,
            0x2882d11a5846ccfaL, 0xf5e0d56325d6d000L, 0x59a0f67b7ae7a5adL,
            0xf01562a268e42c21L, 0x16133104620725ddL, 0x7a9378dcdf599479L,
            0xd9f07bdc76c20a78L, 0x332a4fff07df83daL, 0x976beeefd11659dcL,
            0xc3fcc139e4c6832aL, 0x86130593c7746a6fL, 0x70550dbe5cdde280L,
            0x67211fbaf6b9122dL, 0xe2d06846964b80adL, 0xd55b3c010258ce93L,
            0x5a2507daa032fa13L, 0xaf8618678ae5cd55L, 0xad5a7047e8a139d8L,
            0x8fc110192723cd5eL, 0x50170b4485d7af19L, 0x7c32444652212bf3L,
            0x90e571225cce7360L, 0x9919537c1add41e1L, 0x3a70a8070883029fL,
            0xcc32b418290e2879L, 0xde493e4646077aebL, 0x4d3ad9b55316f970L,
            0x1547de75efe848f4L, 0xe2ead0cc6aab6affL, 0x3dc2f4a9e9b451b4L,
            0xce247654a4de9f51L, 0xbc118f2ba2305503L, 0xb55cd8bdcac2a118L,
            0xb7c97db807c32f38L,
    };

    @Test
    void hashMatchesGoImplementation() {
        String data = "0123456789abcdefghijklmnopqrstuvwxyz";
        for (int i = 0; i < HASH_EXPECTED.length; i++) {
            long actual = Hash.hash(0, data.substring(0, i).getBytes());
            assertEquals(HASH_EXPECTED[i], actual, "hash prefix " + i);
        }
    }

    @Test
    void filterEvalWorks() {
        String payload = """
                [
                  {
                    "filter": [
                      {"op": 1, "child": [1, 2]},
                      {"op": 6, "dtype": 1, "key": "country", "s": "CN"},
                      {"op": 10, "dtype": 2, "key": "age", "i": 18}
                    ],
                    "lyr": [{
                      "name": "L1",
                      "seg": [{
                        "seed": 1,
                        "grp": [{
                          "name": "A",
                          "bm": "%s",
                          "cfg": "cfgA"
                        }]
                      }]
                    }]
                  }
                ]
                """.formatted(fullBitmapBase64());
        List<Model.Experiment> exps = Client.parseExperiments(payload);
        assertTrue(Model.evalExpr(exps.getFirst().filter, Map.of("country", "CN", "age", "20")));
        assertFalse(Model.evalExpr(exps.getFirst().filter, Map.of("country", "US", "age", "20")));
        assertFalse(Model.evalExpr(exps.getFirst().filter, Map.of("country", "CN", "age", "x")));
    }

    @Test
    void richSegmentDispatchWorks() {
        String key = findKey(1, 50, 100);
        String payload = """
                [
                  {
                    "seed": 1,
                    "lyr": [{
                      "name": "layer1",
                      "seg": [
                        {
                          "r": {"a": 0, "b": 50},
                          "seed": 1,
                          "grp": [{"name": "A", "bm": "%s", "cfg": "cfgA"}]
                        },
                        {
                          "r": {"a": 50, "b": 100},
                          "seed": 2,
                          "grp": [{"name": "B", "bm": "%s", "cfg": "cfgB"}]
                        }
                      ]
                    }]
                  }
                ]
                """.formatted(fullBitmapBase64(), fullBitmapBase64());
        Decision decision = Model.getExpConfig(Client.parseExperiments(payload), key, Map.of());
        assertEquals("cfgB", decision.config().get("layer1"));
        assertEquals(List.of("layer1:B"), decision.tags());
    }

    @Test
    void forceHitOverridesDispatch() {
        String payload = """
                [
                  {
                    "lyr": [{
                      "name": "L1",
                      "seg": [{
                        "seed": 1,
                        "grp": [
                          {"name": "A", "bm": "%s", "cfg": "cfgA"},
                          {"name": "F", "bm": "%s", "cfg": "cfgForce"}
                        ]
                      }],
                      "force_hit": {"u2": {"s": 0, "g": 1}}
                    }]
                  }
                ]
                """.formatted(fullBitmapBase64(), fullBitmapBase64());
        Decision decision = Model.getExpConfig(Client.parseExperiments(payload), "u2", Map.of());
        assertEquals("cfgForce", decision.config().get("L1"));
        assertEquals(List.of("L1:F"), decision.tags());
    }

    @Test
    void gsonDecodesBitmapAndDispatches() {
        String payload = """
                [
                  {
                    "lyr": [{
                      "name": "L1",
                      "seg": [{
                        "seed": 1,
                        "grp": [{"name": "G1", "bm": "%s", "cfg": "cfg1"}]
                      }]
                    }]
                  }
                ]
                """.formatted(fullBitmapBase64());
        List<Model.Experiment> exps = Client.parseExperiments(payload);
        assertEquals(1, exps.size());
        assertArrayEquals(fullBitmap(), exps.getFirst().layers.getFirst().segments.getFirst().groups.getFirst().bitmap);
        Decision decision = Model.getExpConfig(exps, "user", Map.of());
        assertEquals("cfg1", decision.config().get("L1"));
        assertEquals(List.of("L1:G1"), decision.tags());
    }

    @Test
    void clientParsesGzipResponse() throws Exception {
        String payload = """
                [
                  {
                    "lyr": [{
                      "name": "L1",
                      "seg": [{
                        "seed": 1,
                        "grp": [{"name": "G1", "bm": "%s", "cfg": "cfg1"}]
                      }]
                    }]
                  }
                ]
                """.formatted(fullBitmapBase64());
        Decision decision = fetchDecision(payload, true);
        assertEquals("cfg1", decision.config().get("L1"));
        assertEquals(List.of("L1:G1"), decision.tags());
    }

    @Test
    void clientParsesPlainResponse() throws Exception {
        String payload = """
                [
                  {
                    "lyr": [{
                      "name": "L1",
                      "seg": [{
                        "seed": 1,
                        "grp": [{"name": "G1", "bm": "%s", "cfg": "cfg1"}]
                      }]
                    }]
                  }
                ]
                """.formatted(fullBitmapBase64());
        Decision decision = fetchDecision(payload, false);
        assertEquals("cfg1", decision.config().get("L1"));
        assertEquals(List.of("L1:G1"), decision.tags());
    }

    @Test
    void clientWrapsInvalidJsonAsIoException() throws Exception {
        OkHttpClient httpClient = new OkHttpClient.Builder()
                .addInterceptor(new StaticResponseInterceptor("{", false))
                .build();
        IOException error = assertThrows(IOException.class,
                () -> Client.create("http://unit.test", 1, "token", 0, httpClient));
        assertEquals("invalid experiment payload", error.getMessage());
        assertInstanceOf(com.google.gson.JsonSyntaxException.class, error.getCause());
    }

    @Test
    void refreshWrapsInvalidBitmapAsIoException() throws Exception {
        String validPayload = """
                [
                  {
                    "lyr": [{
                      "name": "L1",
                      "seg": [{
                        "seed": 1,
                        "grp": [{"name": "G1", "bm": "%s", "cfg": "cfg1"}]
                      }]
                    }]
                  }
                ]
                """.formatted(fullBitmapBase64());
        String invalidPayload = """
                [
                  {
                    "lyr": [{
                      "name": "L1",
                      "seg": [{
                        "seed": 1,
                        "grp": [{"name": "G1", "bm": "!!!", "cfg": "cfg1"}]
                      }]
                    }]
                  }
                ]
                """;
        OkHttpClient httpClient = new OkHttpClient.Builder()
                .addInterceptor(new SequenceResponseInterceptor(validPayload, invalidPayload))
                .build();
        try (Client client = Client.create("http://unit.test", 1, "token", 0, httpClient)) {
            IOException error = assertThrows(IOException.class, client::refresh);
            assertEquals("invalid experiment payload", error.getMessage());
            assertInstanceOf(IllegalArgumentException.class, error.getCause());
        }
    }

    private static String findKey(long seed, long begin, long end) {
        for (int i = 0; i < 10000; i++) {
            String key = "k" + i;
            long slot = Long.remainderUnsigned(Hash.hash(seed, key.getBytes()), 100L);
            if (slot >= begin && slot < end) {
                return key;
            }
        }
        return "";
    }

    private static byte[] fullBitmap() {
        byte[] bitmap = new byte[125];
        for (int i = 0; i < bitmap.length; i++) {
            bitmap[i] = (byte) 0xff;
        }
        return bitmap;
    }

    private static String fullBitmapBase64() {
        return Base64.getEncoder().encodeToString(fullBitmap());
    }

    private static byte[] gzip(String input) throws IOException {
        ByteArrayOutputStream out = new ByteArrayOutputStream();
        try (GZIPOutputStream gzip = new GZIPOutputStream(out)) {
            gzip.write(input.getBytes());
        }
        return out.toByteArray();
    }

    private static Decision fetchDecision(String payload, boolean gzip) throws Exception {
        OkHttpClient httpClient = new OkHttpClient.Builder()
                .addInterceptor(new StaticResponseInterceptor(payload, gzip))
                .build();
        try (Client client = Client.create("http://unit.test", 1, "token", 0, httpClient)) {
            return client.ab("user", Map.of());
        }
    }

    private static final class StaticResponseInterceptor implements Interceptor {
        private final byte[] body;
        private final boolean gzip;

        StaticResponseInterceptor(String payload, boolean gzip) throws IOException {
            this.body = gzip ? gzip(payload) : payload.getBytes();
            this.gzip = gzip;
        }

        @Override
        public Response intercept(Chain chain) {
            Request request = chain.request();
            ResponseBody responseBody = ResponseBody.create(body, MediaType.get("application/json"));
            Response.Builder builder = new Response.Builder()
                    .request(request)
                    .protocol(Protocol.HTTP_1_1)
                    .code(200)
                    .message("OK")
                    .body(responseBody)
                    .header("Content-Type", "application/json");
            if (gzip) {
                builder.header("Content-Encoding", "gzip");
            }
            return builder.build();
        }
    }

    private static final class SequenceResponseInterceptor implements Interceptor {
        private final byte[][] bodies;
        private int index;

        SequenceResponseInterceptor(String... payloads) {
            this.bodies = new byte[payloads.length][];
            for (int i = 0; i < payloads.length; i++) {
                this.bodies[i] = payloads[i].getBytes();
            }
        }

        @Override
        public Response intercept(Chain chain) {
            Request request = chain.request();
            byte[] body = bodies[Math.min(index, bodies.length - 1)];
            index++;
            return new Response.Builder()
                    .request(request)
                    .protocol(Protocol.HTTP_1_1)
                    .code(200)
                    .message("OK")
                    .body(ResponseBody.create(body, MediaType.get("application/json")))
                    .header("Content-Type", "application/json")
                    .build();
        }
    }
}
