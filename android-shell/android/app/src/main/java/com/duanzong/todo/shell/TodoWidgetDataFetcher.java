package com.duanzong.todo.shell;

import android.content.Context;
import android.content.SharedPreferences;
import android.net.Uri;
import android.webkit.CookieManager;

import org.json.JSONArray;
import org.json.JSONObject;

import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.net.HttpURLConnection;
import java.net.URI;
import java.net.URLEncoder;
import java.nio.charset.StandardCharsets;
import java.text.SimpleDateFormat;
import java.util.ArrayList;
import java.util.Calendar;
import java.util.Date;
import java.util.List;
import java.util.Locale;

final class TodoWidgetDataFetcher {
    private static final String PREFS_NAME = "todo_widget";
    private static final String KEY_SNAPSHOT_JSON = "snapshot_json";
    private static final String KEY_FETCHED_AT = "fetched_at";
    private static final int CONNECT_TIMEOUT_MS = 8000;
    private static final int READ_TIMEOUT_MS = 10000;
    static final int MAX_CACHED_TASKS = 24;

    private TodoWidgetDataFetcher() {
    }

    static WidgetSnapshot loadCached(Context context) {
        SharedPreferences prefs = prefs(context);
        String raw = prefs.getString(KEY_SNAPSHOT_JSON, "");
        if (raw == null || raw.trim().isEmpty()) {
            return WidgetSnapshot.empty();
        }

        try {
            WidgetSnapshot snapshot = parseSnapshot(raw);
            snapshot.fetchedAtMillis = prefs.getLong(KEY_FETCHED_AT, 0L);
            snapshot.fromCache = true;
            return snapshot;
        } catch (Exception ignored) {
            return WidgetSnapshot.empty();
        }
    }

    static WidgetSnapshot fetch(Context context) throws Exception {
        String origin = resolveConfiguredServerOrigin(context);
        if (origin == null || origin.trim().isEmpty()) {
            throw new IOException("missing server url");
        }

        String endpoint = origin + "/dashboard/snapshot?date=" + URLEncoder.encode(todayISO(), "UTF-8");
        HttpURLConnection connection = openConnection(endpoint);
        connection.setConnectTimeout(CONNECT_TIMEOUT_MS);
        connection.setReadTimeout(READ_TIMEOUT_MS);
        connection.setRequestMethod("GET");
        connection.setUseCaches(false);
        connection.setRequestProperty("Accept", "application/json");
        connection.setRequestProperty("X-Requested-With", "fetch");
        connection.setRequestProperty("User-Agent", widgetUserAgent(context));

        String cookies = CookieManager.getInstance().getCookie(origin);
        if (cookies != null && !cookies.trim().isEmpty()) {
            connection.setRequestProperty("Cookie", cookies);
        }

        int status = connection.getResponseCode();
        String body = readFully(status >= 400 ? connection.getErrorStream() : connection.getInputStream());
        connection.disconnect();

        if (status == HttpURLConnection.HTTP_UNAUTHORIZED) {
            throw new AuthRequiredException();
        }
        if (status < 200 || status >= 300) {
            throw new IOException("http " + status);
        }

        WidgetSnapshot snapshot = parseSnapshot(body);
        snapshot.fetchedAtMillis = System.currentTimeMillis();
        snapshot.fromCache = false;
        saveSnapshot(context, body, snapshot.fetchedAtMillis);
        return snapshot;
    }

    static WidgetSnapshot completeTask(Context context, String taskID) throws Exception {
        String origin = resolveConfiguredServerOrigin(context);
        if (origin == null || origin.trim().isEmpty()) {
            throw new IOException("missing server url");
        }

        String endpoint = origin + "/tasks/" + URLEncoder.encode(taskID, "UTF-8") + "/complete";
        HttpURLConnection connection = openConnection(endpoint);
        connection.setConnectTimeout(CONNECT_TIMEOUT_MS);
        connection.setReadTimeout(READ_TIMEOUT_MS);
        connection.setRequestMethod("POST");
        connection.setUseCaches(false);
        connection.setDoOutput(true);
        connection.setRequestProperty("Accept", "application/json");
        connection.setRequestProperty("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8");
        connection.setRequestProperty("X-Requested-With", "fetch");
        connection.setRequestProperty("User-Agent", widgetUserAgent(context));

        String cookies = CookieManager.getInstance().getCookie(origin);
        if (cookies != null && !cookies.trim().isEmpty()) {
            connection.setRequestProperty("Cookie", cookies);
        }

        byte[] body = ("return_date=" + URLEncoder.encode(todayISO(), "UTF-8")).getBytes(StandardCharsets.UTF_8);
        connection.getOutputStream().write(body);

        int status = connection.getResponseCode();
        String responseBody = readFully(status >= 400 ? connection.getErrorStream() : connection.getInputStream());
        connection.disconnect();

        if (status == HttpURLConnection.HTTP_UNAUTHORIZED) {
            throw new AuthRequiredException();
        }
        if (status < 200 || status >= 300) {
            throw new IOException("http " + status);
        }

        WidgetSnapshot snapshot = parseSnapshot(responseBody);
        snapshot.fetchedAtMillis = System.currentTimeMillis();
        snapshot.fromCache = false;
        saveSnapshot(context, responseBody, snapshot.fetchedAtMillis);
        return snapshot;
    }

    static String formatFetchedAt(long fetchedAtMillis) {
        if (fetchedAtMillis <= 0L) {
            return "";
        }
        return new SimpleDateFormat("HH:mm", Locale.CHINA).format(new Date(fetchedAtMillis));
    }

    private static SharedPreferences prefs(Context context) {
        return context.getApplicationContext().getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE);
    }

    private static String widgetUserAgent(Context context) {
        return "TodoAndroidWidget/1.0 " + AndroidShellConfig.shellUserAgent(context);
    }

    private static HttpURLConnection openConnection(String endpoint) throws IOException {
        return (HttpURLConnection) URI.create(endpoint).toURL().openConnection();
    }

    private static void saveSnapshot(Context context, String raw, long fetchedAtMillis) throws IOException {
        boolean saved = prefs(context).edit()
            .putString(KEY_SNAPSHOT_JSON, raw)
            .putLong(KEY_FETCHED_AT, fetchedAtMillis)
            .commit();
        if (!saved) {
            throw new IOException("save widget snapshot failed");
        }
    }

    private static WidgetSnapshot parseSnapshot(String raw) throws Exception {
        JSONObject root = new JSONObject(raw);
        WidgetSnapshot snapshot = WidgetSnapshot.empty();
		snapshot.widgetDualColumn = root.optBoolean("widget_dual_column", true);

        JSONArray tasks = root.optJSONArray("focus_tasks");
        if (tasks != null) {
            int count = Math.min(tasks.length(), MAX_CACHED_TASKS);
            for (int index = 0; index < count; index++) {
                JSONObject item = tasks.optJSONObject(index);
                if (item == null) {
                    continue;
                }
                WidgetTask task = new WidgetTask();
                task.id = item.optString("id", "");
                task.title = item.optString("title", "");
                task.kindLabel = item.optString("kind_label", "");
                task.kindClass = item.optString("kind_class", "");
                task.canComplete = item.optBoolean("can_complete", false);
                JSONArray completionUsers = item.optJSONArray("completion_users");
                task.hasCompletionUsers = completionUsers != null && completionUsers.length() > 0;
                if (!task.title.trim().isEmpty()) {
                    snapshot.tasks.add(task);
                }
            }
            snapshot.totalTaskCount = tasks.length();
        }

        JSONObject quote = root.optJSONObject("empty_quote");
        if (quote != null) {
            snapshot.emptyText = quote.optString("text", "");
            snapshot.emptyMeta = quote.optString("meta_line", "");
        }

        return snapshot;
    }

    private static String todayISO() {
        return new SimpleDateFormat("yyyy-MM-dd", Locale.US).format(Calendar.getInstance().getTime());
    }

    private static String resolveConfiguredServerOrigin(Context context) {
        try (InputStream input = context.getAssets().open("capacitor.config.json")) {
            JSONObject config = new JSONObject(readFully(input));
            JSONObject server = config.optJSONObject("server");
            if (server == null) {
                return null;
            }

            Uri uri = Uri.parse(server.optString("url", ""));
            if (!isHttpURL(uri)) {
                return null;
            }
            return uri.getScheme() + "://" + uri.getAuthority();
        } catch (Exception ignored) {
            return null;
        }
    }

    private static boolean isHttpURL(Uri uri) {
        if (uri == null || uri.getScheme() == null || uri.getAuthority() == null) {
            return false;
        }
        return "http".equals(uri.getScheme()) || "https".equals(uri.getScheme());
    }

    private static String readFully(InputStream input) throws IOException {
        if (input == null) {
            return "";
        }

        ByteArrayOutputStream output = new ByteArrayOutputStream();
        byte[] buffer = new byte[4096];
        int read;
        while ((read = input.read(buffer)) != -1) {
            output.write(buffer, 0, read);
        }
        return output.toString(StandardCharsets.UTF_8.name());
    }

    static final class AuthRequiredException extends IOException {
    }

    static final class WidgetSnapshot {
        final List<WidgetTask> tasks = new ArrayList<>();
        int totalTaskCount;
        String emptyText = "";
        String emptyMeta = "";
        long fetchedAtMillis;
        boolean fromCache;
		boolean widgetDualColumn = true;

        static WidgetSnapshot empty() {
            return new WidgetSnapshot();
        }
    }

    static final class WidgetTask {
        String id = "";
        String title = "";
        String kindLabel = "";
        String kindClass = "";
        boolean canComplete;
        boolean hasCompletionUsers;
    }
}
