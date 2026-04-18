import QtQuick
import Quickshell
import qs.Common
import qs.Widgets

FloatingWindow {
    id: root

    property string modalType: "getpin"
    property string desc: ""
    property string prompt: ""
    property string errorText: ""
    property string okLabel: ""
    property string cancelLabel: ""
    property string notOkLabel: ""
    property int timeout: 0
    property bool repeat: false

    signal submitted(string value)
    signal confirmed()
    signal cancelled()
    signal rejectedNotOK()

    property bool disablePopupTransparency: true
    property string passwordInput: ""
    property string repeatInput: ""
    property bool showRepeatField: false
    property bool repeatMismatch: false
    readonly property int inputFieldHeight: Theme.fontSizeMedium + Theme.spacingL * 2
    readonly property bool isGetPin: modalType === "getpin"
    readonly property bool isConfirm: modalType === "confirm"
    readonly property bool isMessage: modalType === "message"
    readonly property string resolvedOkLabel: okLabel || (isGetPin ? "OK" : "OK")
    readonly property string resolvedCancelLabel: cancelLabel || "Cancel"

    function show() {
        passwordInput = "";
        repeatInput = "";
        showRepeatField = false;
        repeatMismatch = false;
        visible = true;
        Qt.callLater(focusInput);
    }

    function close() {
        visible = false;
    }

    function focusInput() {
        if (isGetPin)
            passwordField.forceActiveFocus();
        else
            contentFocusScope.forceActiveFocus();
    }

    function submit() {
        if (isGetPin) {
            if (repeat && !showRepeatField) {
                showRepeatField = true;
                Qt.callLater(() => repeatField.forceActiveFocus());
                return;
            }
            if (repeat && passwordInput !== repeatInput) {
                repeatMismatch = true;
                repeatInput = "";
                Qt.callLater(() => repeatField.forceActiveFocus());
                return;
            }
            submitted(passwordInput);
        } else {
            confirmed();
        }
    }

    function cancel() {
        cancelled();
    }

    objectName: "pinentryModal"
    title: "Pinentry"
    minimumSize: Qt.size(460, isGetPin ? (repeat && showRepeatField ? 260 : 200) : 150)
    maximumSize: minimumSize
    color: Theme.surfaceContainer
    visible: false

    onVisibleChanged: {
        if (visible) {
            Qt.callLater(focusInput);
            if (timeout > 0)
                timeoutTimer.start();
            return;
        }
        passwordInput = "";
        repeatInput = "";
        timeoutTimer.stop();
    }

    Timer {
        id: timeoutTimer
        interval: root.timeout > 0 ? root.timeout * 1000 : 60000
        repeat: false
        onTriggered: cancel()
    }

    FocusScope {
        id: contentFocusScope

        anchors.fill: parent
        focus: true

        Keys.onEscapePressed: event => {
            cancel();
            event.accepted = true;
        }

        Column {
            id: mainColumn
            anchors.fill: parent
            anchors.margins: Theme.spacingM
            spacing: Theme.spacingS

            // Header row with title and close button
            Item {
                width: parent.width
                height: Math.max(titleColumn.implicitHeight, closeBtn.implicitHeight)

                MouseArea {
                    anchors.fill: parent
                    onPressed: windowControls.tryStartMove()
                }

                Column {
                    id: titleColumn
                    anchors.left: parent.left
                    anchors.right: closeBtn.left
                    anchors.rightMargin: Theme.spacingM
                    spacing: Theme.spacingXS

                    StyledText {
                        text: root.title || "Pinentry"
                        font.pixelSize: Theme.fontSizeLarge
                        color: Theme.surfaceText
                        font.weight: Font.Medium
                    }

                    StyledText {
                        text: root.desc
                        font.pixelSize: Theme.fontSizeMedium
                        color: Theme.surfaceTextMedium
                        width: parent.width
                        wrapMode: Text.Wrap
                        maximumLineCount: 3
                        elide: Text.ElideRight
                        visible: text !== ""
                    }
                }

                DankActionButton {
                    id: closeBtn
                    anchors.right: parent.right
                    anchors.top: parent.top
                    iconName: "close"
                    iconSize: Theme.iconSize - 4
                    iconColor: Theme.surfaceText
                    onClicked: cancel()
                }
            }

            // Prompt label
            StyledText {
                text: root.prompt
                font.pixelSize: Theme.fontSizeMedium
                color: Theme.surfaceText
                width: parent.width
                visible: text !== "" && isGetPin
            }

            // Password input
            Rectangle {
                width: parent.width
                height: inputFieldHeight
                radius: Theme.cornerRadius
                color: Theme.surfaceHover
                border.color: passwordField.activeFocus ? Theme.primary : Theme.outlineStrong
                border.width: passwordField.activeFocus ? 2 : 1
                visible: isGetPin

                MouseArea {
                    anchors.fill: parent
                    onClicked: passwordField.forceActiveFocus()
                }

                DankTextField {
                    id: passwordField

                    anchors.fill: parent
                    font.pixelSize: Theme.fontSizeMedium
                    textColor: Theme.surfaceText
                    text: passwordInput
                    showPasswordToggle: true
                    echoMode: passwordVisible ? TextInput.Normal : TextInput.Password
                    placeholderText: ""
                    backgroundColor: "transparent"
                    onTextEdited: passwordInput = text
                    onAccepted: submit()
                }
            }

            // Repeat password input
            Rectangle {
                width: parent.width
                height: inputFieldHeight
                radius: Theme.cornerRadius
                color: Theme.surfaceHover
                border.color: repeatField.activeFocus ? Theme.primary : Theme.outlineStrong
                border.width: repeatField.activeFocus ? 2 : 1
                visible: isGetPin && showRepeatField

                MouseArea {
                    anchors.fill: parent
                    onClicked: repeatField.forceActiveFocus()
                }

                DankTextField {
                    id: repeatField

                    anchors.fill: parent
                    font.pixelSize: Theme.fontSizeMedium
                    textColor: Theme.surfaceText
                    text: repeatInput
                    showPasswordToggle: true
                    echoMode: passwordVisible ? TextInput.Normal : TextInput.Password
                    placeholderText: "Repeat passphrase"
                    backgroundColor: "transparent"
                    onTextEdited: {
                        repeatInput = text;
                        repeatMismatch = false;
                    }
                    onAccepted: submit()
                }
            }

            // Error text
            StyledText {
                text: repeatMismatch ? "Passphrases do not match" : root.errorText
                font.pixelSize: Theme.fontSizeSmall
                color: Theme.error
                width: parent.width
                visible: text !== ""
            }

            // Button row
            Item {
                width: parent.width
                height: 36

                Row {
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: Theme.spacingM

                    // Not OK button (for 3-button confirm)
                    Rectangle {
                        width: Math.max(70, notOkText.contentWidth + Theme.spacingM * 2)
                        height: 36
                        radius: Theme.cornerRadius
                        color: notOkArea.containsMouse ? Theme.surfaceTextHover : "transparent"
                        border.color: Theme.surfaceVariantAlpha
                        border.width: 1
                        visible: isConfirm && root.notOkLabel !== ""

                        StyledText {
                            id: notOkText
                            anchors.centerIn: parent
                            text: root.notOkLabel
                            font.pixelSize: Theme.fontSizeMedium
                            color: Theme.surfaceText
                            font.weight: Font.Medium
                        }

                        MouseArea {
                            id: notOkArea
                            anchors.fill: parent
                            hoverEnabled: true
                            cursorShape: Qt.PointingHandCursor
                            onClicked: root.rejectedNotOK()
                        }
                    }

                    // Cancel button
                    Rectangle {
                        width: Math.max(70, cancelText.contentWidth + Theme.spacingM * 2)
                        height: 36
                        radius: Theme.cornerRadius
                        color: cancelArea.containsMouse ? Theme.surfaceTextHover : "transparent"
                        border.color: Theme.surfaceVariantAlpha
                        border.width: 1
                        visible: !isMessage

                        StyledText {
                            id: cancelText
                            anchors.centerIn: parent
                            text: resolvedCancelLabel
                            font.pixelSize: Theme.fontSizeMedium
                            color: Theme.surfaceText
                            font.weight: Font.Medium
                        }

                        MouseArea {
                            id: cancelArea
                            anchors.fill: parent
                            hoverEnabled: true
                            cursorShape: Qt.PointingHandCursor
                            onClicked: cancel()
                        }
                    }

                    // OK / Submit button
                    Rectangle {
                        width: Math.max(80, okText.contentWidth + Theme.spacingM * 2)
                        height: 36
                        radius: Theme.cornerRadius
                        color: okArea.containsMouse ? Qt.darker(Theme.primary, 1.1) : Theme.primary

                        StyledText {
                            id: okText
                            anchors.centerIn: parent
                            text: resolvedOkLabel
                            font.pixelSize: Theme.fontSizeMedium
                            color: Theme.background
                            font.weight: Font.Medium
                        }

                        MouseArea {
                            id: okArea
                            anchors.fill: parent
                            hoverEnabled: true
                            cursorShape: Qt.PointingHandCursor
                            onClicked: submit()
                        }

                        Behavior on color {
                            ColorAnimation {
                                duration: Theme.shortDuration
                                easing.type: Theme.standardEasing
                            }
                        }
                    }
                }
            }
        }
    }

    FloatingWindowControls {
        id: windowControls
        targetWindow: root
    }
}
