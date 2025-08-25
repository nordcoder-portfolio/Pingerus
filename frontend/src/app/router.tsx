import { createBrowserRouter, RouterProvider } from "react-router-dom";
import { AppLayout } from "./AppLayout";
import { SignIn } from "../pages/auth/SignIn";
import { SignUp } from "../pages/auth/SignUp";
import { ChecksPage } from "../pages/checks/ChecksPage";

const router = createBrowserRouter([
    {
        path: "/",
        element: <AppLayout />,
        children: [
            { index: true, element: <ChecksPage /> },
            { path: "checks", element: <ChecksPage /> },
            { path: "sign-in", element: <SignIn /> },
            { path: "sign-up", element: <SignUp /> }
        ],
    },
]);

export function AppRouter() {
    return <RouterProvider router={router} />;
}
