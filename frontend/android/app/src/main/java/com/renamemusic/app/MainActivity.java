package com.renamemusic.app;

import android.os.Bundle;

import com.getcapacitor.BridgeActivity;

public class MainActivity extends BridgeActivity {
    @Override
    public void onCreate(Bundle savedInstanceState) {
        // Il plugin locale (ponte verso il core Go) va registrato prima che il
        // bridge venga creato in super.onCreate.
        registerPlugin(RenameMusicPlugin.class);
        super.onCreate(savedInstanceState);
    }
}
