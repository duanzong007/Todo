package com.duanzong.todo.shell;

import android.content.Context;
import android.net.Uri;

import org.json.JSONObject;

import java.io.ByteArrayOutputStream;
import java.io.InputStream;
import java.nio.charset.StandardCharsets;

final class AndroidShellConfig {
    private AndroidShellConfig() {
    }

    static String resolveServerOrigin(Context context) {
        try (InputStream input = context.getAssets().open("capacitor.config.json")) {
            ByteArrayOutputStream output = new ByteArrayOutputStream();
            byte[] buffer = new byte[1024];
            int read;
            while ((read = input.read(buffer)) != -1) {
                output.write(buffer, 0, read);
            }
            JSONObject config = new JSONObject(output.toString(StandardCharsets.UTF_8.name()));
            JSONObject server = config.optJSONObject("server");
            if (server == null) {
                return null;
            }
            Uri serverUri = Uri.parse(server.optString("url", ""));
            if (!isHttpURL(serverUri)) {
                return null;
            }
            return serverUri.getScheme() + "://" + serverUri.getAuthority();
        } catch (Exception ignored) {
            return null;
        }
    }

    static boolean isHttpURL(Uri uri) {
        if (uri == null || uri.getScheme() == null || uri.getAuthority() == null) {
            return false;
        }
        return "http".equals(uri.getScheme()) || "https".equals(uri.getScheme());
    }
}
