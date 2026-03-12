package com.github.peterrk.simpleabtest;

import com.google.gson.FieldNamingPolicy;
import com.google.gson.Gson;
import com.google.gson.GsonBuilder;
import com.google.gson.annotations.SerializedName;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.Base64;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;

final class Model {
    private static final Gson GSON = new GsonBuilder()
            .setFieldNamingPolicy(FieldNamingPolicy.LOWER_CASE_WITH_UNDERSCORES)
            .create();

    private Model() {}

    static final class Group {
        String name = "";
        @SerializedName("bm")
        String bitmapBase64 = "";
        @SerializedName("cfg")
        String config = "";
        transient byte[] bitmap = new byte[0];
    }

    static final class SegmentRange {
        @SerializedName("a")
        long begin;
        @SerializedName("b")
        long end;
    }

    static final class Segment {
        @SerializedName("r")
        SegmentRange range = new SegmentRange();
        long seed;
        @SerializedName("grp")
        List<Group> groups = List.of();

        Group locate(String key) {
            long slot = Long.remainderUnsigned(Hash.hash(seed, key.getBytes(StandardCharsets.UTF_8)), 1000L);
            int block = (int) (slot >> 3);
            int mask = 1 << (slot & 7);
            for (Group group : groups) {
                if (block < group.bitmap.length && (group.bitmap[block] & mask) != 0) {
                    return group;
                }
            }
            return null;
        }
    }

    static final class HitIndex {
        @SerializedName("s")
        int seg;
        @SerializedName("g")
        int grp;
    }

    static final class Layer {
        String name = "";
        @SerializedName("seg")
        List<Segment> segments = List.of();
        Map<String, HitIndex> forceHit = Map.of();
    }

    enum OpType {
        NULL,
        AND,
        OR,
        NOT,
        IN,
        NOT_IN,
        EQUAL,
        NOT_EQUAL,
        LESS_THAN,
        LESS_OR_EQUAL,
        GREAT_THAN,
        GREAT_OR_EQUAL;

        static OpType fromCode(int code) {
            if (code < 0 || code >= values().length) {
                return NULL;
            }
            return values()[code];
        }
    }

    enum DataType {
        NULL,
        STR,
        INT,
        FLOAT;

        static DataType fromCode(int code) {
            if (code < 0 || code >= values().length) {
                return NULL;
            }
            return values()[code];
        }
    }

    static final class ExprNode {
        int op;
        int dtype;
        String key = "";
        @SerializedName("s")
        String paramS = "";
        @SerializedName("i")
        long paramI;
        @SerializedName("f")
        double paramF;
        @SerializedName("ss")
        List<String> rawParamSS = List.of();
        List<Integer> child = List.of();
        transient OpType opType = OpType.NULL;
        transient DataType dataType = DataType.NULL;
        transient Set<String> paramSS = Set.of();
    }

    static final class Experiment {
        List<ExprNode> filter = List.of();
        long seed;
        @SerializedName("lyr")
        List<Layer> layers = List.of();
    }

    static List<Experiment> parseExperiments(String payload) {
        Experiment[] exps = GSON.fromJson(payload, Experiment[].class);
        if (exps == null) {
            return List.of();
        }
        ArrayList<Experiment> normalized = new ArrayList<>(exps.length);
        for (Experiment exp : exps) {
            normalized.add(normalizeExperiment(exp));
        }
        return List.copyOf(normalized);
    }

    static Decision getExpConfig(List<Experiment> exps, String key, Map<String, String> ctx) {
        HashMap<String, String> config = new HashMap<>();
        ArrayList<String> tags = new ArrayList<>();
        for (Experiment exp : exps) {
            if (!evalExpr(exp.filter, ctx)) {
                continue;
            }
            if (exp.layers.size() == 1 && exp.layers.getFirst().segments.size() == 1) {
                Layer layer = exp.layers.getFirst();
                Group group = forceHit(layer, key);
                if (group == null) {
                    group = layer.segments.getFirst().locate(key);
                }
                if (group != null) {
                    mark(config, tags, layer, group);
                }
                continue;
            }

            long slot = Long.remainderUnsigned(Hash.hash(exp.seed, key.getBytes(StandardCharsets.UTF_8)), 100L);
            for (Layer layer : exp.layers) {
                Group group = forceHit(layer, key);
                if (group != null) {
                    mark(config, tags, layer, group);
                    continue;
                }
                for (Segment seg : layer.segments) {
                    if (seg.range.begin <= slot && slot < seg.range.end) {
                        group = seg.locate(key);
                        if (group != null) {
                            mark(config, tags, layer, group);
                        }
                        break;
                    }
                }
            }
        }
        return new Decision(Map.copyOf(config), List.copyOf(tags));
    }

    static boolean evalExpr(List<ExprNode> expr, Map<String, String> args) {
        return expr.isEmpty() || evalNode(expr, 0, args);
    }

    private static Experiment normalizeExperiment(Experiment exp) {
        Experiment normalized = exp == null ? new Experiment() : exp;
        normalized.filter = normalized.filter == null ? List.of() : List.copyOf(normalized.filter);
        normalized.layers = normalized.layers == null ? List.of() : normalizeLayers(normalized.layers);
        validateFilter(normalized.filter);
        return normalized;
    }

    private static List<Layer> normalizeLayers(List<Layer> layers) {
        ArrayList<Layer> normalized = new ArrayList<>(layers.size());
        for (Layer layer : layers) {
            Layer item = layer == null ? new Layer() : layer;
            item.name = item.name == null ? "" : item.name;
            item.segments = item.segments == null ? List.of() : normalizeSegments(item.segments);
            item.forceHit = item.forceHit == null ? Map.of() : Map.copyOf(item.forceHit);
            normalized.add(item);
        }
        return List.copyOf(normalized);
    }

    private static List<Segment> normalizeSegments(List<Segment> segments) {
        ArrayList<Segment> normalized = new ArrayList<>(segments.size());
        for (Segment segment : segments) {
            Segment item = segment == null ? new Segment() : segment;
            item.range = item.range == null ? new SegmentRange() : item.range;
            item.groups = item.groups == null ? List.of() : normalizeGroups(item.groups);
            normalized.add(item);
        }
        return List.copyOf(normalized);
    }

    private static List<Group> normalizeGroups(List<Group> groups) {
        ArrayList<Group> normalized = new ArrayList<>(groups.size());
        for (Group group : groups) {
            Group item = group == null ? new Group() : group;
            item.name = item.name == null ? "" : item.name;
            item.config = item.config == null ? "" : item.config;
            item.bitmapBase64 = item.bitmapBase64 == null ? "" : item.bitmapBase64;
            item.bitmap = item.bitmapBase64.isEmpty() ? new byte[0] : Base64.getDecoder().decode(item.bitmapBase64);
            normalized.add(item);
        }
        return List.copyOf(normalized);
    }

    private static void validateFilter(List<ExprNode> nodes) {
        boolean[] used = new boolean[nodes.size()];
        for (int i = 0; i < nodes.size(); i++) {
            ExprNode node = nodes.get(i);
            node.opType = OpType.fromCode(node.op);
            node.dataType = DataType.fromCode(node.dtype);
            node.key = node.key == null ? "" : node.key;
            node.paramS = node.paramS == null ? "" : node.paramS;
            node.child = node.child == null ? List.of() : List.copyOf(node.child);
            node.rawParamSS = node.rawParamSS == null ? List.of() : List.copyOf(node.rawParamSS);
            node.paramSS = node.rawParamSS.isEmpty() ? Set.of() : Set.copyOf(new HashSet<>(node.rawParamSS));
            for (int child : node.child) {
                if (child <= 0 || child >= nodes.size() || used[child]) {
                    throw new IllegalArgumentException("broken filter config");
                }
                used[child] = true;
            }
            switch (node.opType) {
                case AND, OR -> {
                    if (node.child.size() < 2 || node.dataType != DataType.NULL) {
                        throw new IllegalArgumentException("broken filter config");
                    }
                }
                case NOT -> {
                    if (node.child.size() != 1 || node.dataType != DataType.NULL) {
                        throw new IllegalArgumentException("broken filter config");
                    }
                }
                case IN, NOT_IN -> {
                    if (!node.child.isEmpty() || node.key.isEmpty() || node.dataType != DataType.STR || node.paramSS.isEmpty()) {
                        throw new IllegalArgumentException("broken filter config");
                    }
                }
                case EQUAL, NOT_EQUAL, LESS_THAN, LESS_OR_EQUAL, GREAT_THAN, GREAT_OR_EQUAL -> {
                    if (!node.child.isEmpty() || node.key.isEmpty()) {
                        throw new IllegalArgumentException("broken filter config");
                    }
                    if (node.dataType != DataType.STR && node.dataType != DataType.INT && node.dataType != DataType.FLOAT) {
                        throw new IllegalArgumentException("broken filter config");
                    }
                }
                default -> throw new IllegalArgumentException("broken filter config");
            }
        }
    }

    private static void mark(Map<String, String> config, List<String> tags, Layer layer, Group group) {
        config.put(layer.name, group.config);
        tags.add(layer.name + ":" + group.name);
    }

    private static Group forceHit(Layer layer, String key) {
        HitIndex idx = layer.forceHit.get(key);
        if (idx == null || idx.seg < 0 || idx.seg >= layer.segments.size()) {
            return null;
        }
        Segment seg = layer.segments.get(idx.seg);
        if (idx.grp < 0 || idx.grp >= seg.groups.size()) {
            return null;
        }
        return seg.groups.get(idx.grp);
    }

    private static boolean evalNode(List<ExprNode> expr, int index, Map<String, String> args) {
        ExprNode node = expr.get(index);
        return switch (node.opType) {
            case AND -> evalAnd(expr, node.child, args);
            case OR -> evalOr(expr, node.child, args);
            case NOT -> !evalNode(expr, node.child.getFirst(), args);
            case IN -> node.paramSS.contains(args.get(node.key));
            case NOT_IN -> args.containsKey(node.key) && !node.paramSS.contains(args.get(node.key));
            case EQUAL, NOT_EQUAL, LESS_THAN, LESS_OR_EQUAL, GREAT_THAN, GREAT_OR_EQUAL -> compare(node, args);
            case NULL -> false;
        };
    }

    private static boolean evalAnd(List<ExprNode> expr, List<Integer> children, Map<String, String> args) {
        for (int child : children) {
            if (!evalNode(expr, child, args)) {
                return false;
            }
        }
        return true;
    }

    private static boolean evalOr(List<ExprNode> expr, List<Integer> children, Map<String, String> args) {
        for (int child : children) {
            if (evalNode(expr, child, args)) {
                return true;
            }
        }
        return false;
    }

    private static boolean compare(ExprNode node, Map<String, String> args) {
        String raw = args.get(node.key);
        if (raw == null) {
            return false;
        }
        return switch (node.dataType) {
            case STR -> compare(node.opType, raw.compareTo(node.paramS));
            case INT -> {
                try {
                    yield compare(node.opType, Long.compare(Long.parseLong(raw), node.paramI));
                } catch (NumberFormatException ex) {
                    yield false;
                }
            }
            case FLOAT -> {
                try {
                    yield compare(node.opType, Double.compare(Double.parseDouble(raw), node.paramF));
                } catch (NumberFormatException ex) {
                    yield false;
                }
            }
            case NULL -> false;
        };
    }

    private static boolean compare(OpType op, int cmp) {
        return switch (op) {
            case EQUAL -> cmp == 0;
            case NOT_EQUAL -> cmp != 0;
            case LESS_THAN -> cmp < 0;
            case LESS_OR_EQUAL -> cmp <= 0;
            case GREAT_THAN -> cmp > 0;
            case GREAT_OR_EQUAL -> cmp >= 0;
            default -> false;
        };
    }
}
