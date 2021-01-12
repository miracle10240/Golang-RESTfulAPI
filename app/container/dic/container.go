package dic

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/sarulabs/di/v2"
	"github.com/sarulabs/dingo/v4"

	providerPkg "gotham/app/provider"

	controllers "gotham/controllers"
	middlewares "gotham/middlewares"
	repositories "gotham/repositories"
	services "gotham/services"

	gorm "gorm.io/gorm"
)

// C retrieves a Container from an interface.
// The function panics if the Container can not be retrieved.
//
// The interface can be :
// - a *Container
// - an *http.Request containing a *Container in its context.Context
//   for the dingo.ContainerKey("dingo") key.
//
// The function can be changed to match the needs of your application.
var C = func(i interface{}) *Container {
	if c, ok := i.(*Container); ok {
		return c
	}
	r, ok := i.(*http.Request)
	if !ok {
		panic("could not get the container with dic.C()")
	}
	c, ok := r.Context().Value(dingo.ContainerKey("dingo")).(*Container)
	if !ok {
		panic("could not get the container from the given *http.Request in dic.C()")
	}
	return c
}

type builder struct {
	builder *di.Builder
}

// NewBuilder creates a builder that can create a Container.
// You should you NewContainer to create the container directly.
// Using NewBuilder allows you to redefine some di services though.
// This could be used for testing.
// But this behaviour is not safe, so be sure to know what you are doing.
func NewBuilder(scopes ...string) (*builder, error) {
	if len(scopes) == 0 {
		scopes = []string{di.App, di.Request, di.SubRequest}
	}
	b, err := di.NewBuilder(scopes...)
	if err != nil {
		return nil, fmt.Errorf("could not create di.Builder: %v", err)
	}
	provider := &providerPkg.Provider{}
	if err := provider.Load(); err != nil {
		return nil, fmt.Errorf("could not load definitions with the Provider (Provider from gotham/app/provider): %v", err)
	}
	for _, d := range getDiDefs(provider) {
		if err := b.Add(d); err != nil {
			return nil, fmt.Errorf("could not add di.Def in di.Builder: %v", err)
		}
	}
	return &builder{builder: b}, nil
}

// Add adds one or more definitions in the Builder.
// It returns an error if a definition can not be added.
func (b *builder) Add(defs ...di.Def) error {
	return b.builder.Add(defs...)
}

// Set is a shortcut to add a definition for an already built object.
func (b *builder) Set(name string, obj interface{}) error {
	return b.builder.Set(name, obj)
}

// Build creates a Container in the most generic scope.
func (b *builder) Build() *Container {
	return &Container{ctn: b.builder.Build()}
}

// NewContainer creates a new Container.
// If no scope is provided, di.App, di.Request and di.SubRequest are used.
// The returned Container has the most generic scope (di.App).
// The SubContainer() method should be called to get a Container in a more specific scope.
func NewContainer(scopes ...string) (*Container, error) {
	b, err := NewBuilder(scopes...)
	if err != nil {
		return nil, err
	}
	return b.Build(), nil
}

// Container represents a generated dependency injection container.
// It is a wrapper around a di.Container.
//
// A Container has a scope and may have a parent in a more generic scope
// and children in a more specific scope.
// Objects can be retrieved from the Container.
// If the requested object does not already exist in the Container,
// it is built thanks to the object definition.
// The following attempts to get this object will return the same object.
type Container struct {
	ctn di.Container
}

// Scope returns the Container scope.
func (c *Container) Scope() string {
	return c.ctn.Scope()
}

// Scopes returns the list of available scopes.
func (c *Container) Scopes() []string {
	return c.ctn.Scopes()
}

// ParentScopes returns the list of scopes wider than the Container scope.
func (c *Container) ParentScopes() []string {
	return c.ctn.ParentScopes()
}

// SubScopes returns the list of scopes that are more specific than the Container scope.
func (c *Container) SubScopes() []string {
	return c.ctn.SubScopes()
}

// Parent returns the parent Container.
func (c *Container) Parent() *Container {
	if p := c.ctn.Parent(); p != nil {
		return &Container{ctn: p}
	}
	return nil
}

// SubContainer creates a new Container in the next sub-scope
// that will have this Container as parent.
func (c *Container) SubContainer() (*Container, error) {
	sub, err := c.ctn.SubContainer()
	if err != nil {
		return nil, err
	}
	return &Container{ctn: sub}, nil
}

// SafeGet retrieves an object from the Container.
// The object has to belong to this scope or a more generic one.
// If the object does not already exist, it is created and saved in the Container.
// If the object can not be created, it returns an error.
func (c *Container) SafeGet(name string) (interface{}, error) {
	return c.ctn.SafeGet(name)
}

// Get is similar to SafeGet but it does not return the error.
// Instead it panics.
func (c *Container) Get(name string) interface{} {
	return c.ctn.Get(name)
}

// Fill is similar to SafeGet but it does not return the object.
// Instead it fills the provided object with the value returned by SafeGet.
// The provided object must be a pointer to the value returned by SafeGet.
func (c *Container) Fill(name string, dst interface{}) error {
	return c.ctn.Fill(name, dst)
}

// UnscopedSafeGet retrieves an object from the Container, like SafeGet.
// The difference is that the object can be retrieved
// even if it belongs to a more specific scope.
// To do so, UnscopedSafeGet creates a sub-container.
// When the created object is no longer needed,
// it is important to use the Clean method to delete this sub-container.
func (c *Container) UnscopedSafeGet(name string) (interface{}, error) {
	return c.ctn.UnscopedSafeGet(name)
}

// UnscopedGet is similar to UnscopedSafeGet but it does not return the error.
// Instead it panics.
func (c *Container) UnscopedGet(name string) interface{} {
	return c.ctn.UnscopedGet(name)
}

// UnscopedFill is similar to UnscopedSafeGet but copies the object in dst instead of returning it.
func (c *Container) UnscopedFill(name string, dst interface{}) error {
	return c.ctn.UnscopedFill(name, dst)
}

// Clean deletes the sub-container created by UnscopedSafeGet, UnscopedGet or UnscopedFill.
func (c *Container) Clean() error {
	return c.ctn.Clean()
}

// DeleteWithSubContainers takes all the objects saved in this Container
// and calls the Close function of their Definition on them.
// It will also call DeleteWithSubContainers on each child and remove its reference in the parent Container.
// After deletion, the Container can no longer be used.
// The sub-containers are deleted even if they are still used in other goroutines.
// It can cause errors. You may want to use the Delete method instead.
func (c *Container) DeleteWithSubContainers() error {
	return c.ctn.DeleteWithSubContainers()
}

// Delete works like DeleteWithSubContainers if the Container does not have any child.
// But if the Container has sub-containers, it will not be deleted right away.
// The deletion only occurs when all the sub-containers have been deleted manually.
// So you have to call Delete or DeleteWithSubContainers on all the sub-containers.
func (c *Container) Delete() error {
	return c.ctn.Delete()
}

// IsClosed returns true if the Container has been deleted.
func (c *Container) IsClosed() bool {
	return c.ctn.IsClosed()
}

// SafeGetAuthController works like SafeGet but only for AuthController.
// It does not return an interface but a controllers.AuthController.
func (c *Container) SafeGetAuthController() (controllers.AuthController, error) {
	i, err := c.ctn.SafeGet("auth-controller")
	if err != nil {
		var eo controllers.AuthController
		return eo, err
	}
	o, ok := i.(controllers.AuthController)
	if !ok {
		return o, errors.New("could get 'auth-controller' because the object could not be cast to controllers.AuthController")
	}
	return o, nil
}

// GetAuthController is similar to SafeGetAuthController but it does not return the error.
// Instead it panics.
func (c *Container) GetAuthController() controllers.AuthController {
	o, err := c.SafeGetAuthController()
	if err != nil {
		panic(err)
	}
	return o
}

// UnscopedSafeGetAuthController works like UnscopedSafeGet but only for AuthController.
// It does not return an interface but a controllers.AuthController.
func (c *Container) UnscopedSafeGetAuthController() (controllers.AuthController, error) {
	i, err := c.ctn.UnscopedSafeGet("auth-controller")
	if err != nil {
		var eo controllers.AuthController
		return eo, err
	}
	o, ok := i.(controllers.AuthController)
	if !ok {
		return o, errors.New("could get 'auth-controller' because the object could not be cast to controllers.AuthController")
	}
	return o, nil
}

// UnscopedGetAuthController is similar to UnscopedSafeGetAuthController but it does not return the error.
// Instead it panics.
func (c *Container) UnscopedGetAuthController() controllers.AuthController {
	o, err := c.UnscopedSafeGetAuthController()
	if err != nil {
		panic(err)
	}
	return o
}

// AuthController is similar to GetAuthController.
// It tries to find the container with the C method and the given interface.
// If the container can be retrieved, it applies the GetAuthController method.
// If the container can not be retrieved, it panics.
func AuthController(i interface{}) controllers.AuthController {
	return C(i).GetAuthController()
}

// SafeGetAuthService works like SafeGet but only for AuthService.
// It does not return an interface but a services.IAuthService.
func (c *Container) SafeGetAuthService() (services.IAuthService, error) {
	i, err := c.ctn.SafeGet("auth-service")
	if err != nil {
		var eo services.IAuthService
		return eo, err
	}
	o, ok := i.(services.IAuthService)
	if !ok {
		return o, errors.New("could get 'auth-service' because the object could not be cast to services.IAuthService")
	}
	return o, nil
}

// GetAuthService is similar to SafeGetAuthService but it does not return the error.
// Instead it panics.
func (c *Container) GetAuthService() services.IAuthService {
	o, err := c.SafeGetAuthService()
	if err != nil {
		panic(err)
	}
	return o
}

// UnscopedSafeGetAuthService works like UnscopedSafeGet but only for AuthService.
// It does not return an interface but a services.IAuthService.
func (c *Container) UnscopedSafeGetAuthService() (services.IAuthService, error) {
	i, err := c.ctn.UnscopedSafeGet("auth-service")
	if err != nil {
		var eo services.IAuthService
		return eo, err
	}
	o, ok := i.(services.IAuthService)
	if !ok {
		return o, errors.New("could get 'auth-service' because the object could not be cast to services.IAuthService")
	}
	return o, nil
}

// UnscopedGetAuthService is similar to UnscopedSafeGetAuthService but it does not return the error.
// Instead it panics.
func (c *Container) UnscopedGetAuthService() services.IAuthService {
	o, err := c.UnscopedSafeGetAuthService()
	if err != nil {
		panic(err)
	}
	return o
}

// AuthService is similar to GetAuthService.
// It tries to find the container with the C method and the given interface.
// If the container can be retrieved, it applies the GetAuthService method.
// If the container can not be retrieved, it panics.
func AuthService(i interface{}) services.IAuthService {
	return C(i).GetAuthService()
}

// SafeGetDb works like SafeGet but only for Db.
// It does not return an interface but a *gorm.DB.
func (c *Container) SafeGetDb() (*gorm.DB, error) {
	i, err := c.ctn.SafeGet("db")
	if err != nil {
		var eo *gorm.DB
		return eo, err
	}
	o, ok := i.(*gorm.DB)
	if !ok {
		return o, errors.New("could get 'db' because the object could not be cast to *gorm.DB")
	}
	return o, nil
}

// GetDb is similar to SafeGetDb but it does not return the error.
// Instead it panics.
func (c *Container) GetDb() *gorm.DB {
	o, err := c.SafeGetDb()
	if err != nil {
		panic(err)
	}
	return o
}

// UnscopedSafeGetDb works like UnscopedSafeGet but only for Db.
// It does not return an interface but a *gorm.DB.
func (c *Container) UnscopedSafeGetDb() (*gorm.DB, error) {
	i, err := c.ctn.UnscopedSafeGet("db")
	if err != nil {
		var eo *gorm.DB
		return eo, err
	}
	o, ok := i.(*gorm.DB)
	if !ok {
		return o, errors.New("could get 'db' because the object could not be cast to *gorm.DB")
	}
	return o, nil
}

// UnscopedGetDb is similar to UnscopedSafeGetDb but it does not return the error.
// Instead it panics.
func (c *Container) UnscopedGetDb() *gorm.DB {
	o, err := c.UnscopedSafeGetDb()
	if err != nil {
		panic(err)
	}
	return o
}

// Db is similar to GetDb.
// It tries to find the container with the C method and the given interface.
// If the container can be retrieved, it applies the GetDb method.
// If the container can not be retrieved, it panics.
func Db(i interface{}) *gorm.DB {
	return C(i).GetDb()
}

// SafeGetDbPool works like SafeGet but only for DbPool.
// It does not return an interface but a gorm.Dialector.
func (c *Container) SafeGetDbPool() (gorm.Dialector, error) {
	i, err := c.ctn.SafeGet("db-pool")
	if err != nil {
		var eo gorm.Dialector
		return eo, err
	}
	o, ok := i.(gorm.Dialector)
	if !ok {
		return o, errors.New("could get 'db-pool' because the object could not be cast to gorm.Dialector")
	}
	return o, nil
}

// GetDbPool is similar to SafeGetDbPool but it does not return the error.
// Instead it panics.
func (c *Container) GetDbPool() gorm.Dialector {
	o, err := c.SafeGetDbPool()
	if err != nil {
		panic(err)
	}
	return o
}

// UnscopedSafeGetDbPool works like UnscopedSafeGet but only for DbPool.
// It does not return an interface but a gorm.Dialector.
func (c *Container) UnscopedSafeGetDbPool() (gorm.Dialector, error) {
	i, err := c.ctn.UnscopedSafeGet("db-pool")
	if err != nil {
		var eo gorm.Dialector
		return eo, err
	}
	o, ok := i.(gorm.Dialector)
	if !ok {
		return o, errors.New("could get 'db-pool' because the object could not be cast to gorm.Dialector")
	}
	return o, nil
}

// UnscopedGetDbPool is similar to UnscopedSafeGetDbPool but it does not return the error.
// Instead it panics.
func (c *Container) UnscopedGetDbPool() gorm.Dialector {
	o, err := c.UnscopedSafeGetDbPool()
	if err != nil {
		panic(err)
	}
	return o
}

// DbPool is similar to GetDbPool.
// It tries to find the container with the C method and the given interface.
// If the container can be retrieved, it applies the GetDbPool method.
// If the container can not be retrieved, it panics.
func DbPool(i interface{}) gorm.Dialector {
	return C(i).GetDbPool()
}

// SafeGetIsAdminMiddleware works like SafeGet but only for IsAdminMiddleware.
// It does not return an interface but a middlewares.IsAdmin.
func (c *Container) SafeGetIsAdminMiddleware() (middlewares.IsAdmin, error) {
	i, err := c.ctn.SafeGet("is-admin-middleware")
	if err != nil {
		var eo middlewares.IsAdmin
		return eo, err
	}
	o, ok := i.(middlewares.IsAdmin)
	if !ok {
		return o, errors.New("could get 'is-admin-middleware' because the object could not be cast to middlewares.IsAdmin")
	}
	return o, nil
}

// GetIsAdminMiddleware is similar to SafeGetIsAdminMiddleware but it does not return the error.
// Instead it panics.
func (c *Container) GetIsAdminMiddleware() middlewares.IsAdmin {
	o, err := c.SafeGetIsAdminMiddleware()
	if err != nil {
		panic(err)
	}
	return o
}

// UnscopedSafeGetIsAdminMiddleware works like UnscopedSafeGet but only for IsAdminMiddleware.
// It does not return an interface but a middlewares.IsAdmin.
func (c *Container) UnscopedSafeGetIsAdminMiddleware() (middlewares.IsAdmin, error) {
	i, err := c.ctn.UnscopedSafeGet("is-admin-middleware")
	if err != nil {
		var eo middlewares.IsAdmin
		return eo, err
	}
	o, ok := i.(middlewares.IsAdmin)
	if !ok {
		return o, errors.New("could get 'is-admin-middleware' because the object could not be cast to middlewares.IsAdmin")
	}
	return o, nil
}

// UnscopedGetIsAdminMiddleware is similar to UnscopedSafeGetIsAdminMiddleware but it does not return the error.
// Instead it panics.
func (c *Container) UnscopedGetIsAdminMiddleware() middlewares.IsAdmin {
	o, err := c.UnscopedSafeGetIsAdminMiddleware()
	if err != nil {
		panic(err)
	}
	return o
}

// IsAdminMiddleware is similar to GetIsAdminMiddleware.
// It tries to find the container with the C method and the given interface.
// If the container can be retrieved, it applies the GetIsAdminMiddleware method.
// If the container can not be retrieved, it panics.
func IsAdminMiddleware(i interface{}) middlewares.IsAdmin {
	return C(i).GetIsAdminMiddleware()
}

// SafeGetIsVerifiedMiddleware works like SafeGet but only for IsVerifiedMiddleware.
// It does not return an interface but a middlewares.IsVerified.
func (c *Container) SafeGetIsVerifiedMiddleware() (middlewares.IsVerified, error) {
	i, err := c.ctn.SafeGet("is-verified-middleware")
	if err != nil {
		var eo middlewares.IsVerified
		return eo, err
	}
	o, ok := i.(middlewares.IsVerified)
	if !ok {
		return o, errors.New("could get 'is-verified-middleware' because the object could not be cast to middlewares.IsVerified")
	}
	return o, nil
}

// GetIsVerifiedMiddleware is similar to SafeGetIsVerifiedMiddleware but it does not return the error.
// Instead it panics.
func (c *Container) GetIsVerifiedMiddleware() middlewares.IsVerified {
	o, err := c.SafeGetIsVerifiedMiddleware()
	if err != nil {
		panic(err)
	}
	return o
}

// UnscopedSafeGetIsVerifiedMiddleware works like UnscopedSafeGet but only for IsVerifiedMiddleware.
// It does not return an interface but a middlewares.IsVerified.
func (c *Container) UnscopedSafeGetIsVerifiedMiddleware() (middlewares.IsVerified, error) {
	i, err := c.ctn.UnscopedSafeGet("is-verified-middleware")
	if err != nil {
		var eo middlewares.IsVerified
		return eo, err
	}
	o, ok := i.(middlewares.IsVerified)
	if !ok {
		return o, errors.New("could get 'is-verified-middleware' because the object could not be cast to middlewares.IsVerified")
	}
	return o, nil
}

// UnscopedGetIsVerifiedMiddleware is similar to UnscopedSafeGetIsVerifiedMiddleware but it does not return the error.
// Instead it panics.
func (c *Container) UnscopedGetIsVerifiedMiddleware() middlewares.IsVerified {
	o, err := c.UnscopedSafeGetIsVerifiedMiddleware()
	if err != nil {
		panic(err)
	}
	return o
}

// IsVerifiedMiddleware is similar to GetIsVerifiedMiddleware.
// It tries to find the container with the C method and the given interface.
// If the container can be retrieved, it applies the GetIsVerifiedMiddleware method.
// If the container can not be retrieved, it panics.
func IsVerifiedMiddleware(i interface{}) middlewares.IsVerified {
	return C(i).GetIsVerifiedMiddleware()
}

// SafeGetUserController works like SafeGet but only for UserController.
// It does not return an interface but a controllers.UserController.
func (c *Container) SafeGetUserController() (controllers.UserController, error) {
	i, err := c.ctn.SafeGet("user-controller")
	if err != nil {
		var eo controllers.UserController
		return eo, err
	}
	o, ok := i.(controllers.UserController)
	if !ok {
		return o, errors.New("could get 'user-controller' because the object could not be cast to controllers.UserController")
	}
	return o, nil
}

// GetUserController is similar to SafeGetUserController but it does not return the error.
// Instead it panics.
func (c *Container) GetUserController() controllers.UserController {
	o, err := c.SafeGetUserController()
	if err != nil {
		panic(err)
	}
	return o
}

// UnscopedSafeGetUserController works like UnscopedSafeGet but only for UserController.
// It does not return an interface but a controllers.UserController.
func (c *Container) UnscopedSafeGetUserController() (controllers.UserController, error) {
	i, err := c.ctn.UnscopedSafeGet("user-controller")
	if err != nil {
		var eo controllers.UserController
		return eo, err
	}
	o, ok := i.(controllers.UserController)
	if !ok {
		return o, errors.New("could get 'user-controller' because the object could not be cast to controllers.UserController")
	}
	return o, nil
}

// UnscopedGetUserController is similar to UnscopedSafeGetUserController but it does not return the error.
// Instead it panics.
func (c *Container) UnscopedGetUserController() controllers.UserController {
	o, err := c.UnscopedSafeGetUserController()
	if err != nil {
		panic(err)
	}
	return o
}

// UserController is similar to GetUserController.
// It tries to find the container with the C method and the given interface.
// If the container can be retrieved, it applies the GetUserController method.
// If the container can not be retrieved, it panics.
func UserController(i interface{}) controllers.UserController {
	return C(i).GetUserController()
}

// SafeGetUserRepository works like SafeGet but only for UserRepository.
// It does not return an interface but a repositories.IUserRepository.
func (c *Container) SafeGetUserRepository() (repositories.IUserRepository, error) {
	i, err := c.ctn.SafeGet("user-repository")
	if err != nil {
		var eo repositories.IUserRepository
		return eo, err
	}
	o, ok := i.(repositories.IUserRepository)
	if !ok {
		return o, errors.New("could get 'user-repository' because the object could not be cast to repositories.IUserRepository")
	}
	return o, nil
}

// GetUserRepository is similar to SafeGetUserRepository but it does not return the error.
// Instead it panics.
func (c *Container) GetUserRepository() repositories.IUserRepository {
	o, err := c.SafeGetUserRepository()
	if err != nil {
		panic(err)
	}
	return o
}

// UnscopedSafeGetUserRepository works like UnscopedSafeGet but only for UserRepository.
// It does not return an interface but a repositories.IUserRepository.
func (c *Container) UnscopedSafeGetUserRepository() (repositories.IUserRepository, error) {
	i, err := c.ctn.UnscopedSafeGet("user-repository")
	if err != nil {
		var eo repositories.IUserRepository
		return eo, err
	}
	o, ok := i.(repositories.IUserRepository)
	if !ok {
		return o, errors.New("could get 'user-repository' because the object could not be cast to repositories.IUserRepository")
	}
	return o, nil
}

// UnscopedGetUserRepository is similar to UnscopedSafeGetUserRepository but it does not return the error.
// Instead it panics.
func (c *Container) UnscopedGetUserRepository() repositories.IUserRepository {
	o, err := c.UnscopedSafeGetUserRepository()
	if err != nil {
		panic(err)
	}
	return o
}

// UserRepository is similar to GetUserRepository.
// It tries to find the container with the C method and the given interface.
// If the container can be retrieved, it applies the GetUserRepository method.
// If the container can not be retrieved, it panics.
func UserRepository(i interface{}) repositories.IUserRepository {
	return C(i).GetUserRepository()
}

// SafeGetUserService works like SafeGet but only for UserService.
// It does not return an interface but a services.IUserService.
func (c *Container) SafeGetUserService() (services.IUserService, error) {
	i, err := c.ctn.SafeGet("user-service")
	if err != nil {
		var eo services.IUserService
		return eo, err
	}
	o, ok := i.(services.IUserService)
	if !ok {
		return o, errors.New("could get 'user-service' because the object could not be cast to services.IUserService")
	}
	return o, nil
}

// GetUserService is similar to SafeGetUserService but it does not return the error.
// Instead it panics.
func (c *Container) GetUserService() services.IUserService {
	o, err := c.SafeGetUserService()
	if err != nil {
		panic(err)
	}
	return o
}

// UnscopedSafeGetUserService works like UnscopedSafeGet but only for UserService.
// It does not return an interface but a services.IUserService.
func (c *Container) UnscopedSafeGetUserService() (services.IUserService, error) {
	i, err := c.ctn.UnscopedSafeGet("user-service")
	if err != nil {
		var eo services.IUserService
		return eo, err
	}
	o, ok := i.(services.IUserService)
	if !ok {
		return o, errors.New("could get 'user-service' because the object could not be cast to services.IUserService")
	}
	return o, nil
}

// UnscopedGetUserService is similar to UnscopedSafeGetUserService but it does not return the error.
// Instead it panics.
func (c *Container) UnscopedGetUserService() services.IUserService {
	o, err := c.UnscopedSafeGetUserService()
	if err != nil {
		panic(err)
	}
	return o
}

// UserService is similar to GetUserService.
// It tries to find the container with the C method and the given interface.
// If the container can be retrieved, it applies the GetUserService method.
// If the container can not be retrieved, it panics.
func UserService(i interface{}) services.IUserService {
	return C(i).GetUserService()
}
