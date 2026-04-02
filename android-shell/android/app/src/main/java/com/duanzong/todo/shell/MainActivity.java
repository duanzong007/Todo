package com.duanzong.todo.shell;

import android.os.Bundle;
import android.webkit.CookieManager;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewDatabase;
import android.webkit.WebStorage;

import com.getcapacitor.BridgeActivity;

public class MainActivity extends BridgeActivity {
    @Override
    public void onCreate(Bundle savedInstanceState) {
        registerPlugin(SmsBridgePlugin.class);
        super.onCreate(savedInstanceState);
        configureWebViewPersistence();
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

        CookieManager cookieManager = CookieManager.getInstance();
        cookieManager.setAcceptCookie(true);
        cookieManager.setAcceptThirdPartyCookies(webView, true);
        cookieManager.flush();

        WebStorage.getInstance();
        WebViewDatabase.getInstance(this);
    }

    private void flushWebState() {
        CookieManager cookieManager = CookieManager.getInstance();
        cookieManager.flush();
    }
}
