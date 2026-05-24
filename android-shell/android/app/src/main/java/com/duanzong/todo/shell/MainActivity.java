package com.duanzong.todo.shell;

import android.content.Intent;
import android.net.Uri;
import android.os.Bundle;
import android.view.View;
import android.view.WindowManager;
import android.webkit.CookieManager;
import android.webkit.WebResourceRequest;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewDatabase;
import android.webkit.WebStorage;

import androidx.core.content.ContextCompat;
import androidx.core.graphics.Insets;
import androidx.core.view.ViewCompat;
import androidx.core.view.WindowCompat;
import androidx.core.view.WindowInsetsCompat;

import com.getcapacitor.Bridge;
import com.getcapacitor.BridgeActivity;
import com.getcapacitor.BridgeWebViewClient;

import org.json.JSONObject;

import java.io.ByteArrayOutputStream;
import java.io.InputStream;
import java.nio.charset.StandardCharsets;

public class MainActivity extends BridgeActivity {
    private static final String ANDROID_SHELL_USER_AGENT_SUFFIX = " TodoAndroidShell/1.0";
    private static final String SSO_CALLBACK_SCHEME = "todo-shell";
    private static final String SSO_CALLBACK_HOST = "auth";
    private static final String SSO_CALLBACK_PATH = "/sso/callback";

    @Override
    public void onCreate(Bundle savedInstanceState) {
        registerPlugin(SmsBridgePlugin.class);
        super.onCreate(savedInstanceState);
        configureWindowAppearance();
        configureWebViewPersistence();
        configureInAppNavigation();
        configureBridgeInsetsHandling();
        handleSSOCallbackIntent(getIntent());
    }

    @Override
    protected void onNewIntent(Intent intent) {
        super.onNewIntent(intent);
        setIntent(intent);
        handleSSOCallbackIntent(intent);
    }

    @Override
    public void onPause() {
        flushWebState();
        super.onPause();
    }

    @Override
    public void onStop() {
        flushWebState();
        super.onStop();
    }

    private void configureWebViewPersistence() {
        if (getBridge() == null || getBridge().getWebView() == null) {
            return;
        }

        WebView webView = getBridge().getWebView();
        WebSettings settings = webView.getSettings();
        settings.setDomStorageEnabled(true);
        settings.setDatabaseEnabled(true);
        String userAgent = settings.getUserAgentString();
        if (userAgent == null || !userAgent.contains(ANDROID_SHELL_USER_AGENT_SUFFIX.trim())) {
            settings.setUserAgentString((userAgent == null ? "" : userAgent) + ANDROID_SHELL_USER_AGENT_SUFFIX);
        }
        webView.setOverScrollMode(View.OVER_SCROLL_NEVER);
        webView.setVerticalScrollBarEnabled(false);
        webView.setHorizontalScrollBarEnabled(false);

        CookieManager cookieManager = CookieManager.getInstance();
        cookieManager.setAcceptCookie(true);
        cookieManager.setAcceptThirdPartyCookies(webView, true);
        cookieManager.flush();

        WebStorage.getInstance();
        WebViewDatabase.getInstance(this);
    }

    private void configureInAppNavigation() {
        if (getBridge() == null || getBridge().getWebView() == null) {
            return;
        }

        getBridge().setWebViewClient(new InAppSSOWebViewClient(getBridge()));
    }

    private void flushWebState() {
        CookieManager cookieManager = CookieManager.getInstance();
        cookieManager.flush();
    }

    private void configureWindowAppearance() {
        WindowCompat.setDecorFitsSystemWindows(getWindow(), true);
        getWindow().setSoftInputMode(WindowManager.LayoutParams.SOFT_INPUT_ADJUST_RESIZE);
        getWindow().setStatusBarColor(ContextCompat.getColor(this, R.color.splash_background));
        getWindow().setNavigationBarColor(ContextCompat.getColor(this, R.color.splash_background));
    }

    private void configureBridgeInsetsHandling() {
        if (getBridge() == null || getBridge().getWebView() == null) {
            return;
        }

        View container = (View) getBridge().getWebView().getParent();

        ViewCompat.setOnApplyWindowInsetsListener(container, (view, insets) -> {
            view.setPadding(0, 0, 0, 0);

            return new WindowInsetsCompat.Builder(insets)
                .setInsets(WindowInsetsCompat.Type.systemBars() | WindowInsetsCompat.Type.displayCutout(), Insets.NONE)
                .build();
        });

        ViewCompat.requestApplyInsets(container);
    }

    private void handleSSOCallbackIntent(Intent intent) {
        if (intent == null || intent.getData() == null || getBridge() == null || getBridge().getWebView() == null) {
            return;
        }

        handleSSOCallbackUri(getBridge().getWebView(), intent.getData());
    }

    private boolean handleSSOCallbackUri(WebView webView, Uri callbackUri) {
        if (!isSSOCallbackUri(callbackUri)) {
            return false;
        }

        loadSSOCallbackIntoWebView(webView, callbackUri);
        return true;
    }

    private boolean isSSOCallbackUri(Uri uri) {
        return uri != null
            && SSO_CALLBACK_SCHEME.equals(uri.getScheme())
            && SSO_CALLBACK_HOST.equals(uri.getHost())
            && SSO_CALLBACK_PATH.equals(uri.getPath());
    }

    private void loadSSOCallbackIntoWebView(WebView webView, Uri callbackUri) {
        if (webView == null) {
            return;
        }

        String currentUrl = webView.getUrl();
        String serverOrigin = resolveServerOrigin(currentUrl);

        if (serverOrigin == null || serverOrigin.trim().isEmpty()) {
            return;
        }

        Uri.Builder target = new Uri.Builder()
            .scheme(Uri.parse(serverOrigin).getScheme())
            .encodedAuthority(Uri.parse(serverOrigin).getAuthority())
            .path("/auth/sso/callback");
        String query = callbackUri.getEncodedQuery();
        if (query != null && !query.isEmpty()) {
            target.encodedQuery(query);
        }
        webView.loadUrl(target.build().toString());
    }

    private String resolveServerOrigin(String currentUrl) {
        String configuredOrigin = resolveConfiguredServerOrigin();
        if (configuredOrigin != null && !configuredOrigin.trim().isEmpty()) {
            return configuredOrigin;
        }

        Uri currentUri = currentUrl == null ? null : Uri.parse(currentUrl);
        if (isHttpURL(currentUri)) {
            return currentUri.getScheme() + "://" + currentUri.getAuthority();
        }

        return null;
    }

    private String resolveConfiguredServerOrigin() {
        try (InputStream input = getAssets().open("capacitor.config.json")) {
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

    private boolean isHttpURL(Uri uri) {
        if (uri == null || uri.getScheme() == null || uri.getAuthority() == null) {
            return false;
        }
        return "http".equals(uri.getScheme()) || "https".equals(uri.getScheme());
    }

    private class InAppSSOWebViewClient extends BridgeWebViewClient {
        private final Bridge bridge;

        InAppSSOWebViewClient(Bridge bridge) {
            super(bridge);
            this.bridge = bridge;
        }

        @Override
        public boolean shouldOverrideUrlLoading(WebView view, WebResourceRequest request) {
            return shouldOverrideURL(view, request == null ? null : request.getUrl());
        }

        @Override
        public boolean shouldOverrideUrlLoading(WebView view, String url) {
            return shouldOverrideURL(view, url == null ? null : Uri.parse(url));
        }

        private boolean shouldOverrideURL(WebView view, Uri url) {
            if (url == null) {
                return false;
            }

            if (handleSSOCallbackUri(view, url)) {
                return true;
            }

            if (isHttpURL(url)) {
                return false;
            }

            return bridge.launchIntent(url);
        }
    }

}
