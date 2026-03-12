package com.github.peterrk.simpleabtest;

import java.util.List;
import java.util.Map;

public record Decision(Map<String, String> config, List<String> tags) {}
