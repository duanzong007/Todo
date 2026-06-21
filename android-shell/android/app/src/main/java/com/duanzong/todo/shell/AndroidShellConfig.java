package com.duanzong.todo.shell;

import android.content.Context;
import android.content.pm.PackageInfo;
import android.content.pm.PackageManager;
import android.net.Uri;
import android.os.Build;

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

    static String currentVersionName(Context context) {
        if (context == null) {
            return "1.2.0";
        }
        try {
            PackageInfo info = context.getPackageManager().getPackageInfo(context.getPackageName(), 0);
            return info.versionName == null || info.versionName.trim().isEmpty() ? "1.2.0" : info.versionName;
        } catch (PackageManager.NameNotFoundException ignored) {
            return "1.2.0";
        }
    }

    static int currentVersionCode(Context context) {
        if (context == null) {
            return 10200;
        }
        try {
            PackageInfo info = context.getPackageManager().getPackageInfo(context.getPackageName(), 0);
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
                return (int) info.getLongVersionCode();
            }
            return info.versionCode;
        } catch (PackageManager.NameNotFoundException ignored) {
            return 10200;
        }
    }

    static String shellUserAgent(Context context) {
        return "TodoAndroidShell/" + currentVersionName(context);
    }
}
