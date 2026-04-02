package com.duanzong.todo.shell;

import android.os.Bundle;
import android.view.View;
import android.view.WindowManager;
import android.webkit.CookieManager;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewDatabase;
import android.webkit.WebStorage;

import androidx.core.content.ContextCompat;
import androidx.core.graphics.Insets;
import androidx.core.view.ViewCompat;
import androidx.core.view.WindowCompat;
import androidx.core.view.WindowInsetsCompat;

import com.getcapacitor.BridgeActivity;

public class MainActivity extends BridgeActivity {
    @Override
    public void onCreate(Bundle savedInstanceState) {
        registerPlugin(SmsBridgePlugin.class);
        super.onCreate(savedInstanceState);
        configureWindowAppearance();
        configureWebViewPersistence();
        configureBridgeInsetsHandling();
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
}
