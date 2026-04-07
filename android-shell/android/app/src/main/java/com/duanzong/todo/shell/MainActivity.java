package com.duanzong.todo.shell;

import android.net.ConnectivityManager;
import android.net.Network;
import android.net.NetworkCapabilities;
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
    private ConnectivityManager connectivityManager;
    private ConnectivityManager.NetworkCallback networkCallback;
    private final Runnable networkRecoveryRunnable = new Runnable() {
        @Override
        public void run() {
            reloadCurrentPageAfterNetworkChange();
        }
    };

    @Override
    public void onCreate(Bundle savedInstanceState) {
        registerPlugin(SmsBridgePlugin.class);
        super.onCreate(savedInstanceState);
        configureWindowAppearance();
        configureWebViewPersistence();
        configureBridgeInsetsHandling();
    }

    @Override
    public void onStart() {
        super.onStart();
        registerNetworkRecovery();
    }

    @Override
    public void onPause() {
        flushWebState();
        super.onPause();
    }

    @Override
    public void onStop() {
        flushWebState();
        unregisterNetworkRecovery();
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

    private void registerNetworkRecovery() {
        if (networkCallback != null) {
            return;
        }

        connectivityManager = getSystemService(ConnectivityManager.class);
        if (connectivityManager == null) {
            return;
        }

        networkCallback = new ConnectivityManager.NetworkCallback() {
            @Override
            public void onAvailable(Network network) {
                scheduleNetworkRecoveryReload();
            }

            @Override
            public void onCapabilitiesChanged(Network network, NetworkCapabilities networkCapabilities) {
                if (networkCapabilities != null && networkCapabilities.hasCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET)) {
                    scheduleNetworkRecoveryReload();
                }
            }
        };

        connectivityManager.registerDefaultNetworkCallback(networkCallback);
    }

    private void unregisterNetworkRecovery() {
        if (connectivityManager == null || networkCallback == null) {
            return;
        }

        try {
            connectivityManager.unregisterNetworkCallback(networkCallback);
        } catch (IllegalArgumentException ignored) {
            // Callback already unregistered.
        }

        if (getBridge() != null && getBridge().getWebView() != null) {
            getBridge().getWebView().removeCallbacks(networkRecoveryRunnable);
        }

        networkCallback = null;
    }

    private void scheduleNetworkRecoveryReload() {
        if (getBridge() == null || getBridge().getWebView() == null) {
            return;
        }

        WebView webView = getBridge().getWebView();
        webView.removeCallbacks(networkRecoveryRunnable);
        webView.postDelayed(networkRecoveryRunnable, 900);
    }

    private void reloadCurrentPageAfterNetworkChange() {
        if (getBridge() == null || getBridge().getWebView() == null) {
            return;
        }

        WebView webView = getBridge().getWebView();
        String currentUrl = webView.getUrl();
        if (currentUrl == null || currentUrl.trim().isEmpty() || currentUrl.startsWith("about:")) {
            webView.reload();
            return;
        }

        webView.loadUrl(currentUrl);
    }
}
