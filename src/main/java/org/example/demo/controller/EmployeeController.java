package org.example.demo.controller;

import com.baomidou.mybatisplus.core.conditions.query.QueryWrapper;
import com.baomidou.mybatisplus.core.metadata.IPage;
import com.baomidou.mybatisplus.extension.plugins.pagination.Page;

import org.example.demo.model.Employee;
import org.example.demo.mapper.EmployeeMapper;
import org.example.demo.service.EmployeeService;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.context.annotation.Lazy;
import org.springframework.web.bind.annotation.*;

import java.util.List;

@RestController
@RequestMapping("/employees")
public class EmployeeController {

    @Autowired
    @Lazy
    private EmployeeService employeeService;

    @Autowired
    private EmployeeMapper employeeMapper;

    @PostMapping("/save")
    public int save(Employee employee){
        return employeeService.insertEmployee(employee);
    }

    @PostMapping("/saveOrUpdate")
    public void addEmployee(@RequestBody Employee employee) {
        employeeService.addateEmployee(employee); //如果您希望从请求体中获取数据，保留 @RequestBody 是最佳实践。
    }

    // 删除员工
    @DeleteMapping("/remove/{id}")
    public int deleteEmployee(@PathVariable Long id) {
        return employeeService.deleteEmployeeById(id); // 根据 ID 删除员工
    }

    // 根据条件删除员工
    @DeleteMapping("/removeall")
    public int deleteAllEmployees() {
        return employeeService.deleteAllEmployees(); // 根据条件删除员工
    }

    @PutMapping("/update")
    public int updateEmployee(Employee employee){
        return employeeService.updateEmployee(employee);
    }

    @PutMapping("/update/{firstname}")
    public int updateAllEmployee(Employee employee, @PathVariable String firstname) {
        return employeeService.updateAllEmployee(employee, firstname); // 调用 Service 方法更新员工
    }

    @GetMapping("/user/findAll")
    public List<Employee> find() {
        return employeeService.selectAllEmployees();
    }

    @GetMapping("/finddor")
    public List<Employee> findByCond(){
        QueryWrapper<Employee> queryWrapper = new QueryWrapper();
        queryWrapper.eq("last_name","ut");
        return employeeService.selectdor();
    }

    @GetMapping("/get/{id}")
    public Employee get(@PathVariable Long id){
        return employeeService.getEmployeeById(id);
    }

    @GetMapping("/list")
    public List<Employee> list(){
        return employeeService.getAllEmployees();
    }

    @GetMapping("/findByPage")
    public IPage<Employee> findByPage() {
        Page<Employee> page = new Page<>(0, 2);
        return employeeMapper.selectPage(page, null);
    }

    @GetMapping("/page")
    public IPage<Employee> getEmployeeByPage(
            @RequestParam(name = "pageNo", defaultValue = "1") int pageNo,
            @RequestParam(name = "pageSize", defaultValue = "1") int pageSize
    ){
        return employeeService.getEmployeeByPage(pageNo, pageSize);
    }

}