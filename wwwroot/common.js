
function defaultPort(protocol) {
    return protocol == "https:" ? "443" : "80";
}

function protocolFromScheme(scheme) {
    return scheme.toLowerCase() == "https://" ? "https:" : "http:";
}

function authorityHost(authority) {
    var host = authority;

    if (host.charAt(0) == '[') {
        var closingBracket = host.indexOf(']');
        host = closingBracket >= 0 ? host.substring(1, closingBracket) : host;
    }
    else {
        var parts = host.split(':');
        if (parts.length == 2) {
            host = parts[0];
        }
    }

    return host.toLowerCase();
}

function authorityPort(authority, scheme) {
    var port = '';

    if (authority.charAt(0) == '[') {
        var closingBracket = authority.indexOf(']');
        if (closingBracket >= 0 && authority.charAt(closingBracket + 1) == ':') {
            port = authority.substring(closingBracket + 2);
        }
    }
    else {
        var parts = authority.split(':');
        if (parts.length == 2) {
            port = parts[1];
        }
    }

    return port == '' ? defaultPort(protocolFromScheme(scheme)) : port;
}

function browserHost() {
    if (window.location.hostname) {
        return window.location.hostname.toLowerCase();
    }

    return authorityHost(window.location.host);
}

function browserPort() {
    if (window.location.port) {
        return window.location.port;
    }

    var scheme = window.location.protocol == "https:" ? "https://" : "http://";
    return authorityPort(window.location.host, scheme);
}

function isLoopbackHost(host) {
    return host == "localhost" || host.indexOf("127.") == 0 || host == "::1";
}

function splitUniverse(universe) {
    var http ="http://";
    if (window.location.protocol == "https:") {
        //check for encryption and set http or https
        http ="https://";
    }

    var scheme = http;
    var value = universe.replace(/^\s+|\s+$/g, '').replace(/\/+$/g, '');
    var schemeMatch = value.match(/^(https?:\/\/)(.*)$/i);

    if (schemeMatch) {
        scheme = schemeMatch[1];
        value = schemeMatch[2];
    }

    var slash = value.indexOf('/');
    var authority = slash >= 0 ? value.substring(0, slash) : value;
    var path = slash >= 0 ? value.substring(slash) : '';

    return {
        scheme: scheme,
        authority: authority,
        path: path
    };
}

function isCurrentUniverse(universe) {
    var selectedHost = authorityHost(universe.authority);
    var selectedPort = authorityPort(universe.authority, universe.scheme);
    var selectedProtocol = protocolFromScheme(universe.scheme);

    if (selectedProtocol != window.location.protocol) {
        return false;
    }

    if (selectedPort != browserPort()) {
        return false;
    }

    return selectedHost == browserHost() || isLoopbackHost(selectedHost);
}

function universeActionUrl(universe, actionPath) {
    var selected = splitUniverse(universe);

    if (isCurrentUniverse(selected)) {
        return selected.path + actionPath;
    }

    return selected.scheme + selected.authority + selected.path + actionPath;
}

function changeAction(type) {
    if(type != "register" && document.loginForm.universe.value == '') {
        alert('<?php echo loca("LOGIN_NOTCHOSEN");?>');
    }
    else {
        if(type == "login") {
            var url = universeActionUrl(document.loginForm.universe.value, "/game/reg/login2.php");
            document.loginForm.action = url;
        }
        else if (type=="getpw") {
            var url = universeActionUrl(document.loginForm.universe.value, "/game/reg/mail.php");
            document.loginForm.action = url;
            document.loginForm.submit();
        }
        else if(type == "register") {
            var url = universeActionUrl(document.registerForm.universe.value, "/game/reg/newredirect.php");
            document.registerForm.action = url;
        }
    }
}

function printMessage(code, div) {
    var textclass = "";

    if (div == null) {
        div = "statustext";
    }
    switch (code) {
        case "0":
            text = "<?php echo loca("ERROR_0");?>";
            textclass = "fine";
            break;
        case "101":
            text = "<?php echo loca("ERROR_101");?>";
            textclass = "warning";
            break;
        case "102":
            text = "<?php echo loca("ERROR_102");?>";
            textclass = "warning";
            break;
        case "103":
            text = "<?php echo loca("ERROR_103");?>";
            textclass = "warning";
            break;
        case "104":
            text = "<?php echo loca("ERROR_104");?>";
            textclass = "warning";
            break;
        case "105":
            text = "<?php echo loca("ERROR_105");?>";
            textclass = "fine";
            break;
        case "106":
            text = "<?php echo loca("ERROR_106");?>";
            textclass = "fine";
            break;
        case "107":
            text = "<?php echo loca("ERROR_107");?>";
            textclass = "warning";
            break;
        case "108":
            text = "<?php echo loca("ERROR_108");?>";
            textclass = "warning";
            break;
        case "109":
            text = "<?php echo loca("ERROR_109");?>";
            textclass = "warning";
            break;
        case "201":
            text = "<?php echo loca("TIP_201");?>";
            break;
        case "202":
            text = "<?php echo loca("TIP_202");?>";
            break;
        case "203":
            text = "<?php echo loca("TIP_203");?>";
            break;
        case "204":
            text = "<?php echo loca("TIP_204");?>";
            break;
        case "205":
            text = "<?php echo loca("TIP_205");?>";
            break;
        default:
            text = code;
            break;
    }

    if (textclass != "") {
        text = "<span class='" + textclass + "'>" + text + "</span>";
    }
    document.getElementById(div).innerHTML = text;
}
