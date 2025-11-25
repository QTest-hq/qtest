import React from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import Sidebar from '@/components/Sidebar';

// Mock useAuth hook
const mockLogin = jest.fn();
const mockLogout = jest.fn();

jest.mock('@/contexts/AuthContext', () => ({
  useAuth: () => ({
    user: null,
    loading: false,
    login: mockLogin,
    logout: mockLogout,
  }),
}));

describe('Sidebar', () => {
  beforeEach(() => {
    mockLogin.mockClear();
    mockLogout.mockClear();
  });

  it('should render the QTest logo', () => {
    render(<Sidebar />);
    expect(screen.getByText('QTest')).toBeInTheDocument();
    expect(screen.getByText('AI')).toBeInTheDocument();
  });

  it('should render all navigation links', () => {
    render(<Sidebar />);
    expect(screen.getByText('Dashboard')).toBeInTheDocument();
    expect(screen.getByText('Repositories')).toBeInTheDocument();
    expect(screen.getByText('Jobs')).toBeInTheDocument();
    expect(screen.getByText('Tests')).toBeInTheDocument();
    expect(screen.getByText('Coverage')).toBeInTheDocument();
    expect(screen.getByText('Webhooks')).toBeInTheDocument();
    expect(screen.getByText('Team')).toBeInTheDocument();
    expect(screen.getByText('Settings')).toBeInTheDocument();
  });

  it('should show sign in button when not logged in', () => {
    render(<Sidebar />);
    expect(screen.getByText('Sign in with GitHub')).toBeInTheDocument();
  });

  it('should call login when sign in button is clicked', () => {
    render(<Sidebar />);
    fireEvent.click(screen.getByText('Sign in with GitHub'));
    expect(mockLogin).toHaveBeenCalled();
  });

  it('should have correct navigation hrefs', () => {
    render(<Sidebar />);

    const dashboardLink = screen.getByText('Dashboard').closest('a');
    expect(dashboardLink).toHaveAttribute('href', '/');

    const reposLink = screen.getByText('Repositories').closest('a');
    expect(reposLink).toHaveAttribute('href', '/repos');

    const jobsLink = screen.getByText('Jobs').closest('a');
    expect(jobsLink).toHaveAttribute('href', '/jobs');

    const testsLink = screen.getByText('Tests').closest('a');
    expect(testsLink).toHaveAttribute('href', '/tests');

    const coverageLink = screen.getByText('Coverage').closest('a');
    expect(coverageLink).toHaveAttribute('href', '/coverage');

    const webhooksLink = screen.getByText('Webhooks').closest('a');
    expect(webhooksLink).toHaveAttribute('href', '/webhooks');

    const teamLink = screen.getByText('Team').closest('a');
    expect(teamLink).toHaveAttribute('href', '/team');

    const settingsLink = screen.getByText('Settings').closest('a');
    expect(settingsLink).toHaveAttribute('href', '/settings');
  });

  it('should render 9 links (logo + 8 navigation items)', () => {
    render(<Sidebar />);
    const navLinks = screen.getAllByRole('link');
    // Logo/Home + Dashboard, Repos, Jobs, Tests, Coverage, Webhooks, Team, Settings = 9 links
    expect(navLinks.length).toBe(9);
  });
});
