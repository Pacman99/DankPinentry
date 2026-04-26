import QtQuick
import Quickshell
import Quickshell.Io
import qs.Common
import qs.Services
import qs.Modules.Plugins

PluginComponent {
    id: root

    property string pluginId: ""
    property var pluginService: null
    property var popoutService: null

    property var activeModal: null
    property string activeSocketPath: ""

    IpcHandler {
        target: "dankPinentry"

        function prompt(requestJson: string): string {
            try {
                const req = JSON.parse(requestJson);
                root.showModal(req);
                return "OK";
            } catch (e) {
                console.error("PinentryDaemon: failed to parse request:", e);
                return "ERR";
            }
        }
    }

    function showModal(req) {
        // Close any existing modal
        if (activeModal) {
            activeModal.destroy();
            activeModal = null;
        }

        activeSocketPath = req.socket || "";

        activeModal = pinentryModalComponent.createObject(root, {
            "modalType": req.type || "getpin",
            "title": req.title || "Pinentry",
            "desc": req.desc || "",
            "prompt": req.prompt || "",
            "errorText": req.error || "",
            "okLabel": req.okLabel || "",
            "cancelLabel": req.cancelLabel || "",
            "notOkLabel": req.notOkLabel || "",
            "timeout": req.timeout || 0,
            "repeat": req.repeat || false,
        });

        activeModal.submitted.connect(handleSubmit);
        activeModal.confirmed.connect(handleConfirmOK);
        activeModal.cancelled.connect(handleCancel);
        activeModal.rejectedNotOK.connect(handleNotOK);
        activeModal.timedOut.connect(handleTimeout);
        activeModal.show();
    }

    function sendResponse(response) {
        if (!activeSocketPath) {
            console.error("DankPinentry: no socket path");
            return;
        }

        const json = JSON.stringify(response);
        const socketPath = activeSocketPath;
        activeSocketPath = "";

        const sock = responseSocketComponent.createObject(root, {
            "path": socketPath,
            "payload": json,
        });
        sock.connected = true;
    }

    function handleSubmit(value) {
        sendResponse({"type": "pin", "value": value});
        if (activeModal) {
            activeModal.close();
            activeModal.destroy();
            activeModal = null;
        }
    }

    function handleConfirmOK() {
        sendResponse({"type": "ok"});
        if (activeModal) {
            activeModal.close();
            activeModal.destroy();
            activeModal = null;
        }
    }

    function handleCancel() {
        sendResponse({"type": "cancel"});
        if (activeModal) {
            activeModal.close();
            activeModal.destroy();
            activeModal = null;
        }
    }

    function handleNotOK() {
        sendResponse({"type": "notok"});
        if (activeModal) {
            activeModal.close();
            activeModal.destroy();
            activeModal = null;
        }
    }

    function handleTimeout() {
        sendResponse({"type": "timeout"});
        if (activeModal) {
            activeModal.close();
            activeModal.destroy();
            activeModal = null;
        }
    }

    Component {
        id: responseSocketComponent

        Socket {
            property string payload: ""

            onConnectionStateChanged: {
                if (connected) {
                    write(payload + "\n");
                    flush();
                    connected = false;
                    Qt.callLater(destroy);
                }
            }
        }
    }

    property var pinentryModalComponent: null

    Component.onCompleted: {
        pinentryModalComponent = Qt.createComponent("PinentryModal.qml");
        if (pinentryModalComponent.status !== Component.Ready) {
            console.error("DankPinentry: failed to load PinentryModal:", pinentryModalComponent.errorString());
        }
        console.info("DankPinentry: started");
    }

    Component.onDestruction: {
        if (activeModal) {
            activeModal.destroy();
        }
        console.info("PinentryDaemon: stopped");
    }
}
