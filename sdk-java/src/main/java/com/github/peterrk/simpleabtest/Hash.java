package com.github.peterrk.simpleabtest;

import java.nio.ByteBuffer;
import java.nio.ByteOrder;

public final class Hash {
    private Hash() {}

    private static long rot(long x, int k) {
        return (x << k) | (x >>> (64 - k));
    }

    private static final class State {
        long a;
        long b;
        long c;
        long d;

        State(long a, long b, long c, long d) {
            this.a = a;
            this.b = b;
            this.c = c;
            this.d = d;
        }

        void mix() {
            c = rot(c, 50);  c += d;  a ^= c;
            d = rot(d, 52);  d += a;  b ^= d;
            a = rot(a, 30);  a += b;  c ^= a;
            b = rot(b, 41);  b += c;  d ^= b;
            c = rot(c, 54);  c += d;  a ^= c;
            d = rot(d, 48);  d += a;  b ^= d;
            a = rot(a, 38);  a += b;  c ^= a;
            b = rot(b, 37);  b += c;  d ^= b;
            c = rot(c, 62);  c += d;  a ^= c;
            d = rot(d, 34);  d += a;  b ^= d;
            a = rot(a, 5);   a += b;  c ^= a;
            b = rot(b, 36);  b += c;  d ^= b;
        }

        void end() {
            d ^= c;  c = rot(c, 15);  d += c;
            a ^= d;  d = rot(d, 52);  a += d;
            b ^= a;  a = rot(a, 26);  b += a;
            c ^= b;  b = rot(b, 51);  c += b;
            d ^= c;  c = rot(c, 28);  d += c;
            a ^= d;  d = rot(d, 9);   a += d;
            b ^= a;  a = rot(a, 47);  b += a;
            c ^= b;  b = rot(b, 54);  c += b;
            d ^= c;  c = rot(c, 32);  d += c;
            a ^= d;  d = rot(d, 25);  a += d;
            b ^= a;  a = rot(a, 63);  b += a;
        }
    }

    public static long hash(long seed, byte[] key) {
        long magic = 0xdeadbeefdeadbeefL;
        State s = new State(seed, seed, magic, magic);

        ByteBuffer buf = ByteBuffer.wrap(key).order(ByteOrder.LITTLE_ENDIAN);
        while (buf.remaining() >= 32) {
            s.c += buf.getLong();
            s.d += buf.getLong();
            s.mix();
            s.a += buf.getLong();
            s.b += buf.getLong();
        }
        if (buf.remaining() >= 16) {
            s.c += buf.getLong();
            s.d += buf.getLong();
            s.mix();
        }

        s.d += ((long) key.length) << 56;
        switch (key.length & 0xf) {
            case 15:
                s.d += ((long) buf.get(buf.position() + 14) & 0xff) << 48;
            case 14:
                s.d += ((long) buf.get(buf.position() + 13) & 0xff) << 40;
            case 13:
                s.d += ((long) buf.get(buf.position() + 12) & 0xff) << 32;
            case 12:
                s.c += buf.getLong();
                s.d += buf.getInt() & 0xffffffffL;
                break;
            case 11:
                s.d += ((long) buf.get(buf.position() + 10) & 0xff) << 16;
            case 10:
                s.d += ((long) buf.get(buf.position() + 9) & 0xff) << 8;
            case 9:
                s.d += (long) buf.get(buf.position() + 8) & 0xff;
            case 8:
                s.c += buf.getLong();
                break;
            case 7:
                s.c += ((long) buf.get(buf.position() + 6) & 0xff) << 48;
            case 6:
                s.c += ((long) buf.get(buf.position() + 5) & 0xff) << 40;
            case 5:
                s.c += ((long) buf.get(buf.position() + 4) & 0xff) << 32;
            case 4:
                s.c += buf.getInt() & 0xffffffffL;
                break;
            case 3:
                s.c += ((long) buf.get(buf.position() + 2) & 0xff) << 16;
            case 2:
                s.c += ((long) buf.get(buf.position() + 1) & 0xff) << 8;
            case 1:
                s.c += (long) buf.get() & 0xff;
                break;
            case 0:
                s.c += magic;
                s.d += magic;
                break;
            default:
                break;
        }
        s.end();
        return s.a;
    }
}
