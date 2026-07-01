export function authHeader() {
    // return authorization header with jwt token
    let user = {};
    try {
        user = JSON.parse(localStorage.getItem('user'));
    } catch (e) {
        user = {};
    }

    if (user && user.token) {
        return { 'Authorization': 'Bearer ' + user.token };
    } else {
        return {};
    }
}