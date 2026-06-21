package com.duanzong.todo.shell;

import android.app.Activity;
import android.app.AlertDialog;
import android.content.Intent;
import android.net.Uri;
import android.os.Build;
import android.os.Handler;
import android.os.Looper;
import android.provider.Settings;
import android.view.Gravity;
import android.widget.LinearLayout;
import android.widget.ProgressBar;
import android.widget.TextView;

import androidx.core.content.FileProvider;

import com.getcapacitor.JSObject;
import com.getcapacitor.PluginCall;

import java.io.ByteArrayOutputStream;
import java.io.File;
import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.InputStream;
import java.net.HttpURLConnection;
import java.net.URI;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.util.Locale;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

final class AndroidUpdateManager {
    private static final ExecutorService EXECUTOR = Executors.newSingleThreadExecutor();
    private static final Handler MAIN = new Handler(Looper.getMainLooper());
    private static final int CONNECT_TIMEOUT_MS = 8000;
    private static final int READ_TIMEOUT_MS = 15000;
    private static volatile boolean checking;

    private AndroidUpdateManager() {
    }

    static void check(Activity activity, boolean manual, PluginCall call) {
        if (activity == null) {
            resolve(activity, call, false, "当前环境不可用", false);
            return;
        }
        if (checking) {
            if (manual) {
                showMessage(activity, "正在检查更新", "请稍等，当前已有检查任务在进行。");
            }
            resolve(activity, call, false, "正在检查更新", false);
            return;
        }

        checking = true;
        EXECUTOR.execute(() -> {
            try {
                String origin = AndroidShellConfig.resolveServerOrigin(activity);
                if (origin == null || origin.trim().isEmpty()) {
                    throw new IllegalStateException("未配置服务器地址");
                }

                AndroidUpdateInfo info = fetchManifest(origin, AndroidShellConfig.currentVersionName(activity));
                MAIN.post(() -> {
                    checking = false;
                    handleManifest(activity, info, manual, call);
                });
            } catch (Exception exception) {
                MAIN.post(() -> {
                    checking = false;
                    if (manual) {
                        showMessage(activity, "检查更新失败", humanMessage(exception));
                    }
                    resolve(activity, call, false, humanMessage(exception), false);
                });
            }
        });
    }

    static JSObject status(Activity activity) {
        JSObject result = new JSObject();
        result.put("available", true);
        result.put("versionName", AndroidShellConfig.currentVersionName(activity));
        result.put("versionCode", AndroidShellConfig.currentVersionCode(activity));
        result.put("source", "android-native");
        return result;
    }

    private static void handleManifest(Activity activity, AndroidUpdateInfo info, boolean manual, PluginCall call) {
        if (info == null || !info.enabled) {
            if (manual) {
                showMessage(activity, "暂无可用更新", "更新服务暂未配置。");
            }
            resolve(activity, call, true, "更新服务暂未配置", false);
            return;
        }
        int currentVersionCode = AndroidShellConfig.currentVersionCode(activity);
        String currentVersionName = AndroidShellConfig.currentVersionName(activity);
        if (!info.hasUpdate(currentVersionCode)) {
            if (manual) {
                showMessage(activity, "已是最新版本", "当前安卓壳版本为 v" + currentVersionName + "。");
            }
            resolve(activity, call, true, "已是最新版本", false);
            return;
        }

        StringBuilder message = new StringBuilder();
        message.append("当前版本 v").append(currentVersionName)
            .append("\n最新版本 ").append(info.displayVersion()).append("\n\n");
        if (!info.changelog.isEmpty()) {
            for (String item : info.changelog) {
                message.append("• ").append(item).append("\n");
            }
        } else {
            message.append("有新的安卓壳版本可用。");
        }

        AlertDialog.Builder builder = new AlertDialog.Builder(activity)
            .setTitle(info.required ? "需要更新安卓壳" : "发现安卓壳新版本")
            .setMessage(message.toString().trim())
            .setPositiveButton("下载并安装", (dialog, which) -> downloadAndInstall(activity, info, call))
            .setNegativeButton(info.required ? "稍后" : "取消", (dialog, which) -> resolve(activity, call, true, "已取消更新", true));
        builder.show();
    }

    private static AndroidUpdateInfo fetchManifest(String origin, String versionName) throws Exception {
        String endpoint = origin + "/app/update/android";
        HttpURLConnection connection = (HttpURLConnection) URI.create(endpoint).toURL().openConnection();
        connection.setConnectTimeout(CONNECT_TIMEOUT_MS);
        connection.setReadTimeout(READ_TIMEOUT_MS);
        connection.setUseCaches(false);
        connection.setRequestMethod("GET");
        connection.setRequestProperty("Accept", "application/json");
        connection.setRequestProperty("User-Agent", "TodoAndroidShell/" + versionName);

        int status = connection.getResponseCode();
        String body = readFully(status >= 400 ? connection.getErrorStream() : connection.getInputStream());
        connection.disconnect();
        if (status < 200 || status >= 300) {
            throw new IllegalStateException("更新接口返回 HTTP " + status);
        }
        return AndroidUpdateInfo.fromJSON(body);
    }

    private static void downloadAndInstall(Activity activity, AndroidUpdateInfo info, PluginCall call) {
        ProgressUI progress = ProgressUI.show(activity, info.displayVersion());
        EXECUTOR.execute(() -> {
            File apkFile = null;
            try {
                String origin = AndroidShellConfig.resolveServerOrigin(activity);
                String apkURL = resolveURL(origin, info.apkURL);
                apkFile = downloadAPK(activity, apkURL, info, progress);
                verifySHA256(apkFile, info.sha256);
                File finalApkFile = apkFile;
                MAIN.post(() -> {
                    progress.dismiss();
                    installAPK(activity, finalApkFile);
                    resolve(activity, call, true, "已打开系统安装界面", false);
                });
            } catch (Exception exception) {
                if (apkFile != null) {
                    //noinspection ResultOfMethodCallIgnored
                    apkFile.delete();
                }
                MAIN.post(() -> {
                    progress.dismiss();
                    showMessage(activity, "更新失败", humanMessage(exception));
                    resolve(activity, call, false, humanMessage(exception), false);
                });
            }
        });
    }

    private static File downloadAPK(Activity activity, String apkURL, AndroidUpdateInfo info, ProgressUI progress) throws Exception {
        HttpURLConnection connection = (HttpURLConnection) URI.create(apkURL).toURL().openConnection();
        connection.setConnectTimeout(CONNECT_TIMEOUT_MS);
        connection.setReadTimeout(READ_TIMEOUT_MS);
        connection.setUseCaches(false);
        connection.setRequestProperty("Accept", "application/vnd.android.package-archive,*/*");
        connection.setRequestProperty("User-Agent", AndroidShellConfig.shellUserAgent(activity));

        int status = connection.getResponseCode();
        if (status < 200 || status >= 300) {
            throw new IllegalStateException("APK 下载失败 HTTP " + status);
        }

        int total = connection.getContentLength();
        File dir = new File(activity.getCacheDir(), "updates");
        if (!dir.exists() && !dir.mkdirs()) {
            throw new IllegalStateException("无法创建更新缓存目录");
        }
        File apkFile = new File(dir, "Todo-android-shell-" + info.displayVersion() + ".apk");

        try (InputStream input = connection.getInputStream(); FileOutputStream output = new FileOutputStream(apkFile)) {
            byte[] buffer = new byte[32 * 1024];
            int read;
            long downloaded = 0L;
            while ((read = input.read(buffer)) != -1) {
                output.write(buffer, 0, read);
                downloaded += read;
                progress.update(downloaded, total);
            }
        } finally {
            connection.disconnect();
        }
        return apkFile;
    }

    private static void verifySHA256(File file, String expected) throws Exception {
        String actual = sha256(file);
        if (!actual.equalsIgnoreCase(expected)) {
            throw new IllegalStateException("安装包校验失败");
        }
    }

    private static void installAPK(Activity activity, File apkFile) {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O && !activity.getPackageManager().canRequestPackageInstalls()) {
            showMessage(activity, "需要授权安装", "请允许 Todo 安装未知来源应用，授权后再点击检查更新。");
            Intent settings = new Intent(Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES);
            settings.setData(Uri.parse("package:" + activity.getPackageName()));
            activity.startActivity(settings);
            return;
        }

        Uri uri = FileProvider.getUriForFile(activity, activity.getPackageName() + ".fileprovider", apkFile);
        Intent intent = new Intent(Intent.ACTION_VIEW);
        intent.setDataAndType(uri, "application/vnd.android.package-archive");
        intent.addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION);
        intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK);
        activity.startActivity(intent);
    }

    private static String resolveURL(String origin, String rawURL) {
        Uri uri = Uri.parse(rawURL);
        if (uri.getScheme() != null && uri.getAuthority() != null) {
            return rawURL;
        }
        if (rawURL.startsWith("/")) {
            return origin + rawURL;
        }
        return origin + "/" + rawURL;
    }

    private static String sha256(File file) throws Exception {
        MessageDigest digest = MessageDigest.getInstance("SHA-256");
        try (FileInputStream input = new FileInputStream(file)) {
            byte[] buffer = new byte[32 * 1024];
            int read;
            while ((read = input.read(buffer)) != -1) {
                digest.update(buffer, 0, read);
            }
        }
        byte[] hash = digest.digest();
        StringBuilder builder = new StringBuilder(hash.length * 2);
        for (byte b : hash) {
            builder.append(String.format(Locale.ROOT, "%02x", b));
        }
        return builder.toString();
    }

    private static String readFully(InputStream input) throws Exception {
        if (input == null) {
            return "";
        }
        try (InputStream stream = input; ByteArrayOutputStream output = new ByteArrayOutputStream()) {
            byte[] buffer = new byte[4096];
            int read;
            while ((read = stream.read(buffer)) != -1) {
                output.write(buffer, 0, read);
            }
            return output.toString(StandardCharsets.UTF_8.name());
        }
    }

    private static void showMessage(Activity activity, String title, String message) {
        if (activity == null || activity.isFinishing()) {
            return;
        }
        new AlertDialog.Builder(activity)
            .setTitle(title)
            .setMessage(message)
            .setPositiveButton("确定", null)
            .show();
    }

    private static String humanMessage(Exception exception) {
        String message = exception == null ? "" : exception.getMessage();
        return message == null || message.trim().isEmpty() ? "请稍后再试" : message;
    }

    private static void resolve(Activity activity, PluginCall call, boolean ok, String message, boolean cancelled) {
        if (call == null) {
            return;
        }
        JSObject result = new JSObject();
        result.put("ok", ok);
        result.put("message", message);
        result.put("cancelled", cancelled);
        result.put("versionName", AndroidShellConfig.currentVersionName(activity));
        result.put("versionCode", AndroidShellConfig.currentVersionCode(activity));
        call.resolve(result);
    }

    private static final class ProgressUI {
        private final AlertDialog dialog;
        private final ProgressBar progressBar;
        private final TextView progressText;
        private int lastPercent = -1;

        private ProgressUI(AlertDialog dialog, ProgressBar progressBar, TextView progressText) {
            this.dialog = dialog;
            this.progressBar = progressBar;
            this.progressText = progressText;
        }

        static ProgressUI show(Activity activity, String version) {
            LinearLayout layout = new LinearLayout(activity);
            layout.setOrientation(LinearLayout.VERTICAL);
            int padding = dp(activity, 22);
            layout.setPadding(padding, dp(activity, 8), padding, 0);

            TextView text = new TextView(activity);
            text.setGravity(Gravity.START);
            text.setText("准备下载 " + version);
            layout.addView(text, new LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT,
                LinearLayout.LayoutParams.WRAP_CONTENT
            ));

            ProgressBar bar = new ProgressBar(activity, null, android.R.attr.progressBarStyleHorizontal);
            bar.setMax(100);
            LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT,
                LinearLayout.LayoutParams.WRAP_CONTENT
            );
            params.topMargin = dp(activity, 14);
            layout.addView(bar, params);

            AlertDialog dialog = new AlertDialog.Builder(activity)
                .setTitle("下载更新")
                .setView(layout)
                .setCancelable(false)
                .create();
            dialog.show();
            return new ProgressUI(dialog, bar, text);
        }

        void update(long downloaded, int total) {
            if (total <= 0) {
                MAIN.post(() -> progressText.setText("已下载 " + formatBytes(downloaded)));
                return;
            }
            int percent = (int) Math.max(0, Math.min(100, downloaded * 100 / total));
            if (percent == lastPercent) {
                return;
            }
            lastPercent = percent;
            MAIN.post(() -> {
                progressBar.setProgress(percent);
                progressText.setText("已下载 " + percent + "%");
            });
        }

        void dismiss() {
            if (dialog.isShowing()) {
                dialog.dismiss();
            }
        }

        private static String formatBytes(long bytes) {
            if (bytes >= 1024L * 1024L) {
                return String.format(Locale.ROOT, "%.1f MB", bytes / 1024f / 1024f);
            }
            return String.format(Locale.ROOT, "%.0f KB", bytes / 1024f);
        }

        private static int dp(Activity activity, int value) {
            return Math.round(value * activity.getResources().getDisplayMetrics().density);
        }
    }
}
