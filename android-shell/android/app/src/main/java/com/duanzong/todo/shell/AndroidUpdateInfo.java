package com.duanzong.todo.shell;

import org.json.JSONArray;
import org.json.JSONObject;

import java.util.ArrayList;
import java.util.List;

final class AndroidUpdateInfo {
    boolean enabled;
    String versionName;
    int versionCode;
    String apkURL;
    String sha256;
    boolean required;
    List<String> changelog = new ArrayList<>();

    static AndroidUpdateInfo fromJSON(String raw) throws Exception {
        JSONObject root = new JSONObject(raw);
        AndroidUpdateInfo info = new AndroidUpdateInfo();
        info.enabled = root.optBoolean("enabled", false);
        info.versionName = root.optString("version_name", "").trim();
        info.versionCode = root.optInt("version_code", 0);
        info.apkURL = root.optString("apk_url", "").trim();
        info.sha256 = root.optString("sha256", "").trim().toLowerCase();
        info.required = root.optBoolean("required", false);

        JSONArray items = root.optJSONArray("changelog");
        if (items != null) {
            for (int i = 0; i < items.length(); i++) {
                String item = items.optString(i, "").trim();
                if (!item.isEmpty()) {
                    info.changelog.add(item);
                }
            }
        }
        return info;
    }

    boolean hasUpdate(int currentVersionCode) {
        return enabled && versionCode > currentVersionCode && !apkURL.isEmpty() && !sha256.isEmpty();
    }

    String displayVersion() {
        return versionName.startsWith("v") ? versionName : "v" + versionName;
    }
}
