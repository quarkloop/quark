package com.quarkloop.quark.runtime.engine.store;

import java.util.List;
import java.util.Optional;

public interface SourceRepository {
    void saveSource(String namespace, String name, String source);
    Optional<String> getSource(String namespace, String name);
    List<SourceEntry> listSources();
    record SourceEntry(String namespace, String name) {}
}
